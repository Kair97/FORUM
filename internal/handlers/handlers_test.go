package handlers

import (
	"context"
	"database/sql"
	"forum/internal/auth"
	"forum/internal/database"
	"forum/internal/middleware"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Init(dbPath, "../../migrations/schema.sql")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedUser(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()

	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	result, err := db.Exec(
		`INSERT INTO users (email, username, password_hash, role)
		 VALUES (?, ?, ?, ?)`,
		username+"@test.com", username, hash, "user",
	)
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read user id: %v", err)
	}
	return id
}

func requestWithUser(req *http.Request, userID int64) *http.Request {
	ctx := req.Context()
	ctx = contextWithUser(ctx, userID)
	return req.WithContext(ctx)
}

func contextWithUser(ctx context.Context, userID int64) context.Context {
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	return context.WithValue(ctx, middleware.RoleKey, "user")
}

func TestRegisterPOSTRejectsMissingFields(t *testing.T) {
	db := newTestDB(t)
	form := url.Values{
		"email":    {""},
		"username": {"alice"},
		"password": {"secret123"},
	}

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	RegisterPOST(db)(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLoginPOSTRejectsWrongPassword(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "alice")

	form := url.Values{
		"email":    {"alice@test.com"},
		"password": {"wrong-password"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()

	LoginPOST(db)(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestIndexGETRedirectsGuestForCreatedFilter(t *testing.T) {
	db := newTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/?filter=created", nil)
	recorder := httptest.NewRecorder()

	IndexGET(db)(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/login" {
		t.Errorf("Location = %q, want %q", location, "/login")
	}
}

func TestCreateCommentPOSTRejectsEmptyComment(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice")

	form := url.Values{
		"post_id": {"1"},
		"content": {""},
	}
	req := httptest.NewRequest(http.MethodPost, "/comments", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, userID)
	recorder := httptest.NewRecorder()

	CreateCommentPOST(db)(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestToggleLikePOSTRejectsExternalRedirect(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice")
	categoryID := firstCategoryID(t, db)
	postID := seedPost(t, db, userID, categoryID)

	form := url.Values{
		"target_type": {"post"},
		"target_id":   {strconv.FormatInt(postID, 10)},
		"is_like":     {"1"},
		"redirect":    {"https://evil.example"},
	}
	req := httptest.NewRequest(http.MethodPost, "/like", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, userID)
	recorder := httptest.NewRecorder()

	ToggleLikePOST(db)(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/" {
		t.Errorf("Location = %q, want %q", location, "/")
	}
}

func firstCategoryID(t *testing.T, db *sql.DB) int64 {
	t.Helper()

	var categoryID int64
	if err := db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID); err != nil {
		t.Fatalf("failed to fetch category id: %v", err)
	}
	return categoryID
}

func seedPost(t *testing.T, db *sql.DB, userID, categoryID int64) int64 {
	t.Helper()

	postID, err := database.CreatePost(db, userID, "Title", "Content", "", []int64{categoryID})
	if err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}
	return postID
}
