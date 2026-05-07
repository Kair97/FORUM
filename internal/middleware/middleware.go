package middleware

import (
	"context"
	"database/sql"
	"forum/internal/auth"
	"net/http"
)

// Custom type for context keys in this package
// It prevents collisions with context keys from other packages that might also use "userID"
type contextKey string

// UserIDKey is the key used to store and retrieve the user ID from context.
const UserIDKey contextKey = "userID"

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

		// Session is valid. Attach the userID to the request context
		// so the handler could identify who is making the request
		ctx := context.WithValue(r.Context(), UserIDKey, userID)

		// Call the next handler with the enriched context.
		// r.WithContext(ctx) returns a shallow copy of r with new context -
		// it does not modify the original request.
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
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// No valid session - call the handler anyway without a userID in context
		next.ServeHTTP(w, r)
	}
}
