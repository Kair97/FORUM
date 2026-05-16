package utils

import (
	"bytes"
	"context"
	"forum/internal/middleware"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(42))

	userID, ok := GetUserID(req.WithContext(ctx))

	if !ok {
		t.Fatal("expected user id to be present")
	}
	if userID != 42 {
		t.Errorf("user id = %d, want %d", userID, 42)
	}
}

func TestGetRole(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), middleware.RoleKey, "admin")

	if role := GetRole(req.WithContext(ctx)); role != "admin" {
		t.Errorf("role = %q, want %q", role, "admin")
	}
}

func TestRateLimiterAllowsFiveAttemptsThenBlocksSixth(t *testing.T) {
	limiter := &rateLimiter{attempts: make(map[string][]time.Time)}

	for i := 0; i < 5; i++ {
		if !limiter.Allow("127.0.0.1") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}

	if limiter.Allow("127.0.0.1") {
		t.Error("sixth attempt should be blocked")
	}
}

func TestSaveUploadImageNoFile(t *testing.T) {
	req := newEmptyMultipartRequest(t)

	path, err := SaveUploadImage(req, "image")

	if err != nil {
		t.Fatalf("SaveUploadImage returned error: %v", err)
	}
	if path != "" {
		t.Errorf("path = %q, want empty string", path)
	}
}

func TestSaveUploadImageRejectsInvalidType(t *testing.T) {
	req := newMultipartRequest(t, "image", "note.txt", []byte("plain text"))

	_, err := SaveUploadImage(req, "image")

	if err == nil {
		t.Fatal("expected invalid file type error")
	}
}

func newEmptyMultipartRequest(t *testing.T) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newMultipartRequest(t *testing.T, fieldName, fileName string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
