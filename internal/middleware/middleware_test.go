package middleware

import (
	"database/sql"
	"forum/internal/auth"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"forum/internal/database"
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

func seedUser(t *testing.T, db *sql.DB, role string) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO users (email, username, password_hash, role)
		 VALUES (?, ?, ?, ?)`,
		role+"@test.com", role, "fakehash", role,
	)
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read seeded user id: %v", err)
	}
	return id
}

func requestWithSession(t *testing.T, db *sql.DB, userID int64) *http.Request {
	t.Helper()

	recorder := httptest.NewRecorder()
	if err := auth.CreateSession(recorder, db, userID); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}
	return req
}

func TestRequireAuthRedirectsGuest(t *testing.T) {
	db := newTestDB(t)

	called := false
	handler := RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}, db)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	handler(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/login" {
		t.Errorf("Location = %q, want %q", location, "/login")
	}
	if called {
		t.Error("next handler should not run for a guest")
	}
}

func TestRequireAuthAddsUserToContext(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "user")
	req := requestWithSession(t, db, userID)

	var gotUserID int64
	var gotRole string
	handler := RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = r.Context().Value(UserIDKey).(int64)
		gotRole, _ = r.Context().Value(RoleKey).(string)
	}, db)

	handler(httptest.NewRecorder(), req)

	if gotUserID != userID {
		t.Errorf("user id in context = %d, want %d", gotUserID, userID)
	}
	if gotRole != "user" {
		t.Errorf("role in context = %q, want %q", gotRole, "user")
	}
}

func TestOptionalAuthStillCallsNextForGuest(t *testing.T) {
	db := newTestDB(t)

	called := false
	handler := OptionalAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := r.Context().Value(UserIDKey).(int64); ok {
			t.Error("guest request should not have a user id in context")
		}
	}, db)

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("next handler should run for a guest")
	}
}

func TestRequireModeratorBlocksNormalUser(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "user")
	req := requestWithSession(t, db, userID)

	called := false
	handler := RequireModerator(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}, db)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if called {
		t.Error("next handler should not run for a normal user")
	}
}

func TestRequireModeratorAllowsModerator(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "moderator")
	req := requestWithSession(t, db, userID)

	called := false
	handler := RequireModerator(func(w http.ResponseWriter, r *http.Request) {
		called = true
		role, _ := r.Context().Value(RoleKey).(string)
		if role != "moderator" {
			t.Errorf("role in context = %q, want %q", role, "moderator")
		}
	}, db)

	handler(httptest.NewRecorder(), req)

	if !called {
		t.Error("next handler should run for a moderator")
	}
}

func TestRequireAdminBlocksModerator(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "moderator")
	req := requestWithSession(t, db, userID)

	recorder := httptest.NewRecorder()
	handler := RequireAdmin(func(w http.ResponseWriter, r *http.Request) {}, db)
	handler(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if got := recorder.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want %q", got, "DENY")
	}
	if got := recorder.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want %q", got, "strict-origin-when-cross-origin")
	}
}
