package handlers

import (
	"database/sql"
	"forum/internal/auth"
	"forum/internal/models"
	"net/http"
	"strings"
)

func RegisterGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Registration is coming soon in phase 8"))
	}
}

func RegisterPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// r.ParseFrom() parses the request body as form data
		// And must be called before r.FormValu() will work
		// returns error if the body is malformed
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(r.FormValue("email"))
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if email == "" || username == "" || password == "" {
			http.Error(w, "All fields are reqruired", http.StatusBadRequest)
			return
		}

		if len(password) < 6 {
			http.Error(w, "Password is too short (len(password)>=6)", http.StatusBadRequest)
			return
		}

		// Check weather email already used or not
		var existingID int64
		err := db.QueryRow(
			"SELECT id FROM users WHERE email = ?", email,
		).Scan(&existingID)

		if err == nil {
			http.Error(w, "Email already registered", http.StatusBadRequest)
			return
		}
		if err != sql.ErrNoRows {
			http.Error(w, "Interval Server Error", http.StatusInternalServerError)
			return
		}

		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Insert new user to database
		result, err := db.Exec(
			"INSERT INTO users (email, username, password_hash, role) VALUES (?, ?, ?, ?)", email, username, passwordHash, "user",
		)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		userID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if err := auth.CreateSession(w, db, userID); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)

	}
}

func LoginGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Login page is coming in phase-8"))
	}
}

func LoginPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")

		if email == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		var user models.User
		err := db.QueryRow(
			`Select id, email, username, password_hash,role
			from users where email = ?`,
			email,
		).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role)

		if err == sql.ErrNoRows {
			// No user found with this email
			// Important: we use same error message as wrong password
			// So that it prevents user enumeration --> an attacker cannot tell
			// whether email exists or password was wrong
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Verify the password
		if err := auth.CheckPassword(password, user.PasswordHash); err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		// Password is correct. Create a new session for this user
		if err := auth.CreateSession(w, db, user.ID); err != nil {
			http.Error(w, "Interncal Server Error", http.StatusInternalServerError)
			return
		}

		// Login is successful - redirect to homepage
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Logout deletes the session and clears the cookie.
// We use POST for logout - not GET - because logout is state changing action.
func LogoutPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// DeleteSession handles both the DB deletion and cookie clearing.
		if err := auth.DeleteSession(w, r, db); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Redirect to homepage after logout
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
