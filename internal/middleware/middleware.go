package middleware

import (
	"context"
	"database/sql"
	"forum/internal/auth"
	"net/http"
)

// Custom type for context keys in this package
// It prevents collisions with context keys from other packages that might also use "userID"
type ContextKey string

const UserIDKey ContextKey = "userID"
const RoleKey ContextKey = "role"

func RequireAuth(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// ValidateSession reads the cookie ans checks the database
		// Returns the userID if valid, error if not.
		userID, err := auth.ValidateSession(r, db)
		if err != nil {
			// Session is missing, invalid or expired
			// Redirect to /login - do not call the next handler
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		var role string
		db.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)

		// Session is valid. Attach the userID to the request context
		// so the handler could identify who is making the request
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, RoleKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))

	}
}

// OptionalAuth is middleware for routes visible to everyone,
// but where we still want to know if the user is logged in or not.
// Example: if user is logged in then we show him "Login/Logout" buttons.
// Unlike RequireAuth, this never redirects - it just attaches the
// userID if available and always calls the next handler.
func OptionalAuth(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to validate
		userID, err := auth.ValidateSession(r, db)
		if err == nil {
			// Session is valid - attach userID to context
			var role string
			db.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, RoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// No valid session - call the handler anyway without a userID in context
		next.ServeHTTP(w, r)
	}
}

func RequireModerator(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := auth.ValidateSession(r, db)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var role string
		err = db.QueryRow(
			"SELECT role FROM users WHERE id = ?", userID,
		).Scan(&role)
		if err != nil || (role != "moderator" && role != "admin") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, ContextKey("role"), role)
		next.ServeHTTP(w, r.WithContext(ctx))

	}
}

func RequireAdmin(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, err := auth.ValidateSession(r, db)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var role string
		err = db.QueryRow(
			"SELECT role FROM users WHERE id = ?", userID,
		).Scan(&role)

		if err != nil || (role != "admin") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, ContextKey("role"), role)

		next.ServeHTTP(w, r.WithContext(ctx))

	}
}
