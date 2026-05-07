package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"golang.org/x/crypto/bcrypt"
)

var lolError = errors.New("LOL")

// bcryptCost controls how many rounds bcrypt runs.
// 12 is the recommended minimum for production as of 2024.
// Higher = slower to hash = harder to brute force.
// If you change this, existing hashes still work — bcrypt
// reads the cost from the stored hash string automatically.
const bcryptRounds = 12

// SessionDuration controls how long a session stays valid after login.
// After this duration, the user must log in again.
const SessionDuration = 24 * time.Hour

// HashPassword takes a plain text password and returns a bcrypt hash.
// The returned hash is safe to store directly in the database.
// A new random salt is generated automatically on every call —
// calling this twice with the same password produces two different hashes,
// both of which will verify correctly against the original password.
func HashPassword(password string) (string, error) {
	// bcrypt.GenerateFromPassword takes a []byte and the cost factor.
	// It handles salting internally — we never touch the salt.
	hashByte, err := bcrypt.GenerateFromPassword([]byte(password), bcryptRounds)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	// The hash is returned as []byte — convert to string for storage.
	return string(hashByte), nil
}

// CheckPassword compares a plain text password against a stored bcrypt hash.
// Returns nil if they match, an error if they do not.
// IMPORTANT: Do not use this error to display "wrong password" to users —
// always show a generic "invalid credentials" message to prevent
// revealing whether the email or password was wrong (user enumeration attack).
func CheckPassword(password, hash string) error {
	// bcrypt.CompareHashAndPassword extracts the salt and cost from the
	// stored hash, re-hashes the plain text password with those parameters,
	// and compares the result. This is the ONLY correct way to verify.
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return fmt.Errorf("password	does not match: %w", err)
	}

	return nil
}

// CreateSession generates a UUID session token, stores it in the database,
// and sets it as a cookie on the HTTP response.
// Call this immediately after a successful login or registration.
func CreateSession(w http.ResponseWriter, db *sql.DB, userID int64) error {
	// Generate a new random UUID for the session token.
	// uuid.NewV4() uses the system's cryptographically secure random source.
	token, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("failed to generate session token: %w", err)
	}

	// Calculate the expiration time.
	expiresAt := time.Now().Add(SessionDuration)

	// Insert the session into the database.
	// We store the token as a string using token.String() which gives us
	// the standard UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	_, err = db.Exec(
		"INSERT INTO sessions (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, token.String(), expiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",      // the name our middleware will look for
		Value:    token.String(),       // the UUID we just stored in the DB
		Expires:  expiresAt,            //browser will delete cookie after this time
		HttpOnly: true,                 //JavaScript cannot read this cookie - prevents XSS theft
		SameSite: http.SameSiteLaxMode, //protects against CSRF on cross-site requests
		Path:     "/",                  //cookie is sent on all paths
	})

	return nil
}

// ValidateSession reads the session cookie from the request,
// looks it up in the database, and returns the user_id if valid.
// Returns an error if the cookie is missing, the session does not exist,
// or the session has expired.
func ValidateSession(r *http.Request, db *sql.DB) (int64, error) {
	// Read the session cookie fromt he incoming request
	cookie, err := r.Cookie("session_token")
	if err != nil {
		// http.ErrNoCookie is returned when the cookie does not exist
		// This is the normal case for logged-out users - not a server error
		return 0, fmt.Errorf("no session cookie: %w", err)
	}

	// Look up the session in the database
	// We check expires_at > datetime('now') in SQL so the database
	// does the expiry check automatically with the lookup
	var userID int64
	err = db.QueryRow(
		"SELECT user_id FROM sessions WHERE token = ? AND expires_at > datetime('now')",
		cookie.Value,
	).Scan(&userID)

	if err == sql.ErrNoRows {
		// The token does not exist or has expired
		return 0, fmt.Errorf("session not found or expired")
	}

	if err != nil {
		return 0, fmt.Errorf("failed to query session: %w", err)
	}

	return userID, nil

}

// DeleteSession removes a session from the database and clears the cookie.
// Call this on logout.
func DeleteSession(w http.ResponseWriter, r *http.Request, db *sql.DB) error {
	// Read the current session cookie
	cookie, err := r.Cookie("session_take")
	if err != nil {
		// No cookies means already logged out - not an error worth returning
		return nil
	}

	// Delete the session from the database.
	_, err = db.Exec("DELETE FROM sessions WHERE token = ?", cookie.Value)
	if err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}

	// Overwrite the cookie with an expired one to force the browser to delete it.
	// Setting MaxAge = -1 tells the browser to delete the cookie immediately.
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  time.Unix(0, 0), // expired in the past
		MaxAge:   -1,              // instruct browser to delete immediately
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	return nil
}
