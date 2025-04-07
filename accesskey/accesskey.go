package accesskey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// AccessKey represents an access key in the system
type AccessKey struct {
	ID          string         `json:"id"`
	SecretKey   string         `json:"secret_key"`
	AccessKey   string         `json:"access_key"`
	UserID      int64          `json:"user_id"`
	Status      string         `json:"status"`
	Permissions []*Permissions `json:"permissions"`
	CreatedAt   time.Time      `json:"created_at"`
	LastUsedAt  time.Time      `json:"last_used_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

// Role represents a role in the system
type Role struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Permissions []*Permissions `json:"permissions"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Permissions struct {
	Resources []string `json:"resources"`
	Actions   []string `json:"actions"`
	Effect    string   `json:"effect"`
}

// DB is the database connection
var DB *sql.DB

// InitDB initializes the database connection
func InitDB(dataSourceName string) error {
	var err error
	DB, err = sql.Open("mysql", dataSourceName)
	if err != nil {
		return err
	}

	// Check the connection
	err = DB.Ping()
	if err != nil {
		return err
	}

	return nil
}

// GenerateAccessKeyPair generates a new access key pair
func GenerateAccessKeyPair() (string, string, error) {
	// 使用时间戳和随机数生成较短的 accessKeyID
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}
	accessKeyID := fmt.Sprintf("%x%x", timestamp, randomBytes)

	// 使用随机字节生成 accessKeySecret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	accessKeySecret := base64.StdEncoding.EncodeToString(secretBytes)

	return accessKeyID, accessKeySecret, nil
}

// CreateAccessKey creates a new access key for a user with specified permissions
func CreateAccessKey(userID int64, permissions string) (string, string, error) {
	if DB == nil {
		return "", "", errors.New("database not initialized")
	}

	// Generate access key pair
	id, secret, err := GenerateAccessKeyPair()
	if err != nil {
		return "", "", err
	}

	// Create access key in database
	_, err = DB.Exec(
		"INSERT INTO access_keys (id, secret_key, access_key, user_id, permissions) VALUES (?, ?, ?, ?, ?)",
		id,
		secret,
		id, // Access key is the same as ID for simplicity
		userID,
		permissions,
	)

	if err != nil {
		return "", "", err
	}

	return id, secret, nil
}

// AssignRoleToAccessKey assigns a role to an access key
func AssignRoleToAccessKey(accessKeyID string, roleID int) error {
	if DB == nil {
		return errors.New("database not initialized")
	}

	// Check if access key exists
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM access_keys WHERE access_key = ?)", accessKeyID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return errors.New("access key not found")
	}

	// Check if role exists
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM roles WHERE id = ?)", roleID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return errors.New("role not found")
	}

	// Assign role to access key
	_, err = DB.Exec(
		"INSERT INTO access_key_roles (access_key_id, role_id) VALUES (?, ?)",
		accessKeyID,
		roleID,
	)

	return err
}

// ValidateAccessKey validates an access key
func ValidateAccessKey(accessKeyID string) (bool, error) {
	if DB == nil {
		return false, errors.New("database not initialized")
	}

	// Check if access key exists and is active
	var status string
	err := DB.QueryRow(
		"SELECT status FROM access_keys WHERE access_key = ?",
		accessKeyID,
	).Scan(&status)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	// Update last used timestamp
	_, err = DB.Exec(
		"UPDATE access_keys SET last_used_at = ? WHERE access_key = ?",
		time.Now(),
		accessKeyID,
	)

	if err != nil {
		return false, err
	}

	return status == "active", nil
}

// GetAccessKeyPermissions gets the permissions for an access key
func GetAccessKeyPermissions(accessKeyID string) ([]*Permissions, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}

	// Get access key permissions
	var permissions string
	err := DB.QueryRow(
		"SELECT permissions FROM access_keys WHERE access_key = ?",
		accessKeyID,
	).Scan(&permissions)

	if err != nil {
		return nil, err
	}

	// Get role permissions
	rows, err := DB.Query(
		`SELECT r.permissions 
		FROM roles r 
		JOIN access_key_roles akr ON r.id = akr.role_id 
		WHERE akr.access_key_id = ?`,
		accessKeyID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse access key permissions
	var keyPerms []*Permissions
	err = json.Unmarshal([]byte(permissions), &keyPerms)
	if err != nil {
		return nil, err
	}

	// Create a map to store unique permissions
	permMap := make(map[string]*Permissions)

	// Add access key permissions
	for _, perm := range keyPerms {
		key := fmt.Sprintf("%v-%v-%v", perm.Resources, perm.Actions, perm.Effect)
		permMap[key] = perm
	}

	// Add role permissions
	for rows.Next() {
		var rolePermissions string
		err = rows.Scan(&rolePermissions)
		if err != nil {
			return nil, err
		}

		var rolePerms []*Permissions
		err = json.Unmarshal([]byte(rolePermissions), &rolePerms)
		if err != nil {
			return nil, err
		}

		for _, perm := range rolePerms {
			key := fmt.Sprintf("%v-%v-%v", perm.Resources, perm.Actions, perm.Effect)
			permMap[key] = perm
		}
	}

	// Convert map values to slice
	var allPermissions []*Permissions
	for k, perm := range permMap {
		log.Printf("key: %s, perm: %+v", k, perm)
		allPermissions = append(allPermissions, perm)
	}

	return allPermissions, nil
}

// GenerateSignature generates an HMAC-SHA256 signature for a request
func GenerateSignature(accessKeySecret string, stringToSign string) string {
	h := hmac.New(sha256.New, []byte(accessKeySecret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies an HMAC-SHA256 signature for a request
func VerifySignature(accessKeyID string, stringToSign string, signature string) (bool, error) {
	if DB == nil {
		return false, errors.New("database not initialized")
	}

	// Get access key secret
	var secretKey string
	err := DB.QueryRow(
		"SELECT secret_key FROM access_keys WHERE access_key = ? AND status = 'active'",
		accessKeyID,
	).Scan(&secretKey)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	// Generate signature
	expectedSignature := GenerateSignature(secretKey, stringToSign)

	// Compare signatures
	return expectedSignature == signature, nil
}

// GetUserAccessKeys gets all access keys for a user
func GetUserAccessKeys(userID int64) ([]AccessKey, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}

	// Get all access keys for user
	rows, err := DB.Query(
		"SELECT id, secret_key, access_key, user_id, status, permissions, created_at, last_used_at, expires_at FROM access_keys WHERE user_id = ?",
		userID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accessKeys []AccessKey
	for rows.Next() {
		var ak AccessKey
		var lastUsedAt, expiresAt sql.NullTime

		err = rows.Scan(
			&ak.ID,
			&ak.SecretKey,
			&ak.AccessKey,
			&ak.UserID,
			&ak.Status,
			&ak.Permissions,
			&ak.CreatedAt,
			&lastUsedAt,
			&expiresAt,
		)

		if err != nil {
			return nil, err
		}

		if lastUsedAt.Valid {
			ak.LastUsedAt = lastUsedAt.Time
		}

		if expiresAt.Valid {
			ak.ExpiresAt = expiresAt.Time
		}

		accessKeys = append(accessKeys, ak)
	}

	return accessKeys, nil
}
