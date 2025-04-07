package accesskey

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// SignatureParams contains parameters needed for signature generation
type SignatureParams struct {
	AccessKeyID string
	Method      string
	Path        string
	QueryParams map[string]string
	Headers     map[string]string
	Timestamp   string
	Content     []byte
}

// GenerateStringToSign generates the string to be signed
func GenerateStringToSign(params SignatureParams) string {
	// 1. Start with HTTP method
	parts := []string{params.Method}

	// 2. Add path
	parts = append(parts, params.Path)

	// 3. Add sorted query parameters
	queryKeys := make([]string, 0, len(params.QueryParams))
	for k := range params.QueryParams {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)

	queryParts := make([]string, 0, len(queryKeys))
	for _, k := range queryKeys {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, params.QueryParams[k]))
	}
	parts = append(parts, strings.Join(queryParts, "&"))

	// 4. Add timestamp
	parts = append(parts, params.Timestamp)

	// 5. Add content hash if available
	if len(params.Content) > 0 {
		contentHash := sha256.Sum256(params.Content)
		parts = append(parts, base64.StdEncoding.EncodeToString(contentHash[:]))
	} else {
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// SignRequest signs an HTTP request with HMAC-SHA256
func SignRequest(req *http.Request, accessKeyID string, accessKeySecret string, content []byte) {
	// Add required headers
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	req.Header.Set("X-Access-Key-ID", accessKeyID)
	req.Header.Set("X-Timestamp", timestamp)

	// Prepare signature parameters
	queryParams := make(map[string]string)
	for k, v := range req.URL.Query() {
		if len(v) > 0 {
			queryParams[k] = v[0]
		}
	}

	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	params := SignatureParams{
		AccessKeyID: accessKeyID,
		Method:      req.Method,
		Path:        req.URL.Path,
		QueryParams: queryParams,
		Headers:     headers,
		Timestamp:   timestamp,
		Content:     content,
	}

	// Generate string to sign
	stringToSign := GenerateStringToSign(params)

	// Generate signature
	signature := GenerateSignature(accessKeySecret, stringToSign)

	// Add signature to request
	req.Header.Set("X-Signature", signature)
}

// VerifyRequestSignature verifies the signature of an HTTP request
func VerifyRequestSignature(req *http.Request, content []byte) (bool, error) {
	// Get access key ID from request
	accessKeyID := req.Header.Get("X-Access-Key-ID")
	if accessKeyID == "" {
		return false, fmt.Errorf("missing X-Access-Key-ID header")
	}

	// Get timestamp from request
	timestamp := req.Header.Get("X-Timestamp")
	if timestamp == "" {
		return false, fmt.Errorf("missing X-Timestamp header")
	}

	// Get signature from request
	signature := req.Header.Get("X-Signature")
	if signature == "" {
		return false, fmt.Errorf("missing X-Signature header")
	}

	// Prepare signature parameters
	queryParams := make(map[string]string)
	for k, v := range req.URL.Query() {
		if len(v) > 0 {
			queryParams[k] = v[0]
		}
	}

	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 && k != "X-Signature" {
			headers[k] = v[0]
		}
	}

	params := SignatureParams{
		AccessKeyID: accessKeyID,
		Method:      req.Method,
		Path:        req.URL.Path,
		QueryParams: queryParams,
		Headers:     headers,
		Timestamp:   timestamp,
		Content:     content,
	}

	// Generate string to sign
	stringToSign := GenerateStringToSign(params)

	// Verify signature
	return VerifySignature(accessKeyID, stringToSign, signature)
}

// matchPathPattern checks if a request path matches a pattern with wildcards
func matchPathPattern(pattern, path string) bool {
	// If pattern ends with *, it's a prefix match
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}

	// Exact match
	return pattern == path
}

// hasPermission checks if the given permissions allow access to the specified method and path
func hasPermission(perms []*Permissions, method, path string) bool {
	// Check if permissions are empty
	if perms == nil || len(perms) == 0 {
		return false
	}

	// Track if we found any matching rules
	foundMatch := false
	finalAllow := false

	for _, perm := range perms {
		resources := perm.Resources
		actions := perm.Actions
		effect := perm.Effect

		effect = strings.ToLower(effect)
		if effect != "allow" && effect != "deny" {
			continue
		}
		// Check if method is allowed
		methodAllowed := false
		for _, actionStr := range actions {
			actionStr = strings.ToUpper(actionStr)
			if actionStr == strings.ToUpper(method) || actionStr == "*" {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			continue
		}
		// Check if path is allowed
		pathAllowed := false
		for _, resource := range resources {
			if matchPathPattern(resource, path) {
				pathAllowed = true
				break
			}
		}
		if pathAllowed {
			foundMatch = true
			// For deny rules, return false immediately
			if effect == "deny" {
				return false
			}
			// For allow rules, mark it but continue checking for deny rules
			finalAllow = true
		}
	}
	// Return true only if we found at least one match and the final result is allow
	return foundMatch && finalAllow
}

// CreateMiddleware creates a middleware for signature verification
func CreateMiddleware(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			// Reset the body for subsequent reads
			r.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		// Sign the request
		SignRequest(r, "14789", "wen", body)
		// Skip signature verification for certain paths if needed
		// if r.URL.Path == "/public/endpoint" {
		// 	next.ServeHTTP(w, r)
		// 	return
		// }

		// Verify signature in the server
		valid, err := VerifyRequestSignature(r, body)
		if err != nil || !valid {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid signature"))
			return
		}

		// Verify whether the access key is available
		accessKeyID := r.Header.Get("X-Access-Key-ID")
		valid, err = ValidateAccessKey(accessKeyID)
		if err != nil || !valid {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid access key"))
			return
		}

		// Get all permissions for the access key
		permissions, err := GetAccessKeyPermissions(accessKeyID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error getting permissions"))
			return
		}

		// Check if the access key has permission to access the endpoint
		// This is a simplified example, you would need to implement your own permission checking logic
		//var perms map[string]interface{}
		//err = json.Unmarshal([]byte(permissions), &perms)
		//if err != nil {
		//	w.WriteHeader(http.StatusInternalServerError)
		//	w.Write([]byte("Error parsing permissions"))
		//	return
		//}

		// Example permission check
		log.Printf("Checking permission for %s /// %s /// %s", permissions, r.Method, r.URL.Path)
		if !hasPermission(permissions, r.Method, r.URL.Path) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Insufficient permissions"))
			return
		}

		// Call the next handler
		f.ServeHTTP(w, r)
	})

}
