package utils

import (
	"fmt"
	"forum/internal/middleware"
	"forum/internal/render"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

// Utils - utility functions
// Small reusable helper functions that don’t belong to a specific feature.

// GetUserID reads the authenticated user's ID from the request context.
// Returns the userID and true if the user is logged in.
// Returns 0 and false if the user is not logged in.
// Handlers use this to check auth status and get the current user's ID
func GetUserID(r *http.Request) (int64, bool) {
	// r.Context().Value() returns interface {} - we must type assert it.
	// The comma-ok pattern: if the key does not exist or the type is wrong,
	// ok will be false instead of panicking.
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	return userID, ok
}

// RenderTemplate executes a pre-parsed template from the render package's cache.
// Templates are parsed once at startup — calling this is just a lookup and execute.
func RenderTemplate(w http.ResponseWriter, pagePath string, data interface{}) {
	render.Template(w, pagePath, data)
}

// Shifting left by 1 is like multiplying by 2.
// So shifting left by 20 it means: 10 * 2^20
// 10 << 20 = 10 * 1,048,576 = 10,485,760
// Which is about 10MB
const maxUploadSize = 10 << 20 // 10 MB

var allowedMIMETypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// SaveUploadedImage reads an uploaded image from the request,
// validates its type and size, saves it to the uploads directory,
// and returns the public URL path to the saved file.
// Returns an empty string and nil error if no file was uploaded.
// Returns an error if the file is invalid or cannot be saved.
func SaveUploadImage(r *http.Request, fieldName string) (string, error) {
	// Retrieve the file from the multipart form.
	// r.FormFile returns the file, its header info, and any error.
	file, header, err := r.FormFile(fieldName)
	if err == http.ErrMissingFile {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("failed to read uploaded file: %w", err)
	}

	defer file.Close()

	if header.Size > maxUploadSize {
		return "", fmt.Errorf("file is too large: max 10MB: %w", err)
	}

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read file header: %w", err)
	}

	mimeType := http.DetectContentType(buffer)

	ext, allowed := allowedMIMETypes[mimeType]
	if !allowed {
		return "", fmt.Errorf("invalid file type: %w", err)
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("failed to reset file reader: %w", err)
	}

	id, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("failed to generate filename: %w", err)
	}

	filename := id.String() + ext

	uploadDir := "volume/uploaded_imgs"

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	fullPath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file on disk: %w", err)
	}

	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return "", fmt.Errorf("failed to save the file: %w", err)
	}

	return "/uploads/" + filename, nil
}

func GetRole(r *http.Request) string {
	role, _ := r.Context().Value(middleware.ContextKey("role")).(string)
	return role
}

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var LoginLimiter = &rateLimiter{
	attempts: make(map[string][]time.Time),
}

func (rl *rateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	window := 60 * time.Second
	max := 5

	var recent []time.Time
	for _, t := range rl.attempts[ip] {
		if now.Sub(t) < window {
			recent = append(recent, t)
		}
	}

	rl.attempts[ip] = recent

	if len(recent) >= max {
		return false
	}

	rl.attempts[ip] = append(rl.attempts[ip], now)

	return true

}

// RenderError renders the error page with the given HTTP status code.
func RenderError(w http.ResponseWriter, status int) {
	render.Error(w, status)
}
