package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"test/accesskey"
)

func main() {
	//Initialize database connection (commented out for demonstration)
	err := accesskey.InitDB("root:123456@tcp(172.29.249.210:3306)/test")
	if err != nil {
		fmt.Println("Error initializing database:", err)
		return
	}

	// Demo: Generate an access key pair
	id, secret, err := accesskey.GenerateAccessKeyPair()
	if err != nil {
		fmt.Println("Error generating access key pair:", err)
		return
	}
	fmt.Println("Generated Access Key ID:", id)
	fmt.Println("Generated Secret Key:", secret)

	// Demo: Create permissions for a RAM account
	permissions := map[string]interface{}{
		"resources": []string{"api/v1/users/*", "api/v1/products/read"},
		"actions":   []string{"GET", "POST"},
		"effect":    "allow",
	}

	// Convert permissions to JSON
	permissionsJSON, err := json.Marshal(permissions)
	if err != nil {
		fmt.Println("Error marshaling permissions:", err)
		return
	}

	// Demo: Create an access key for a user with specific permissions
	// In a real application, this would be stored in the database
	fmt.Println("\nCreating access key with permissions:")
	fmt.Println(string(permissionsJSON))

	handler := http.HandlerFunc(testHandler)
	http.Handle("/api/v1/users/123", accesskey.CreateMiddleware(handler))

	http.ListenAndServe(":8080", nil)
	// In a real application, you would also:
	// 1. Create roles with specific permissions
	// 2. Assign roles to access keys
	// 3. Validate access keys and check permissions for each request
}
func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Hello, World!")
}

// GenerateAccessKeyPair is now implemented in the accesskey package
// This function is kept for backward compatibility
func GenerateAccessKeyPair() (string, string, error) {
	return accesskey.GenerateAccessKeyPair()
}

// CreateAccessKey is now implemented in the accesskey package
// This function is kept for backward compatibility
func CreateAccessKey(userID int64, permissions string) (string, string, error) {
	return accesskey.CreateAccessKey(userID, permissions)
}

// AssignRoleToAccessKey is now implemented in the accesskey package
// This function is kept for backward compatibility
func AssignRoleToAccessKey(accessKeyID string, roleID int) error {
	return accesskey.AssignRoleToAccessKey(accessKeyID, roleID)
}

// ValidateAccessKey is now implemented in the accesskey package
// This function is kept for backward compatibility
func ValidateAccessKey(accessKeyID string) (bool, error) {
	return accesskey.ValidateAccessKey(accessKeyID)
}
