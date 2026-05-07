package utils

import (
	"forum/internal/middleware"
	"net/http"
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
