package handlers

import (
	"database/sql"
	"forum/internal/auth"
	"forum/internal/models"
	"forum/internal/utils"
	"net/http"
	"strings"
)

const login = "web/templates/login.html"
const register = "web/templates/register.html"

func RegisterGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.RenderTemplate(w, register, models.Template{})
	}
}

func RegisterPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(r.FormValue("email"))
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		// showError is a helper that re-renders the register form with
		// the error message AND the user's already-typed email and username,
		// so they don't have to retype everything after a mistake.
		showError := func(msg string) {
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error:        msg,
				FormEmail:    email,
				FormUsername: username,
			})
		}

		if email == "" || username == "" || password == "" {
			showError("All fields are required")
			return
		}

		if len(email) > 254 {
			showError("Email must be 254 characters or less")
			return
		}
		if len(username) > 30 {
			showError("Username must be 30 characters or less")
			return
		}
		// bcrypt silently truncates passwords over 72 bytes,
		// so we reject them explicitly so the user knows the limit.
		if len(password) > 72 {
			showError("Password must be 72 characters or less")
			return
		}
		if len(password) < 6 {
			showError("Password is too short!")
			return
		}

		// Check whether email is already registered.
		var existingID int64
		err := db.QueryRow(
			"SELECT id FROM users WHERE email = ?", email,
		).Scan(&existingID)

		if err == nil {
			showError("Email already registered")
			return
		}
		if err != sql.ErrNoRows {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		// Check whether username is already taken.
		err = db.QueryRow(
			"SELECT id FROM users WHERE username = ?", username,
		).Scan(&existingID)

		if err == nil {
			showError("Username already taken")
			return
		}
		if err != sql.ErrNoRows {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		result, err := db.Exec(
			"INSERT INTO users (email, username, password_hash, role) VALUES (?, ?, ?, ?)",
			email, username, passwordHash, "user",
		)
		if err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		userID, err := result.LastInsertId()
		if err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		if err := auth.CreateSession(w, db, userID); err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func LoginGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.RenderTemplate(w, login, models.Template{})
	}
}

func LoginPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(r.FormValue("email"))

		ip := r.RemoteAddr
		if i := strings.LastIndex(ip, ":"); i != -1 {
			ip = ip[:i]
		}

		// showError re-renders the login form with the error message
		// AND keeps the user's email in the field so they don't retype it.
		// Note: we never keep the password — that is a security best practice.
		showError := func(status int, msg string) {
			w.WriteHeader(status)
			utils.RenderTemplate(w, login, models.Template{
				Error:     msg,
				FormEmail: email,
			})
		}

		if !utils.LoginLimiter.Allow(ip) {
			showError(http.StatusTooManyRequests, "Too many login attempts. Please wait a minute and try again.")
			return
		}

		password := r.FormValue("password")

		if email == "" || password == "" {
			showError(http.StatusBadRequest, "All fields are required!")
			return
		}

		var user models.User
		err := db.QueryRow(
			`SELECT id, email, username, password_hash, role
			 FROM users WHERE email = ?`,
			email,
		).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role)

		if err == sql.ErrNoRows {
			// Same error message for "email not found" and "wrong password"
			// so an attacker cannot tell which one is the real problem.
			showError(http.StatusUnauthorized, "Invalid email or password")
			return
		}
		if err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		if err := auth.CheckPassword(password, user.PasswordHash); err != nil {
			showError(http.StatusUnauthorized, "Invalid email or password")
			return
		}

		if err := auth.CreateSession(w, db, user.ID); err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// LogoutPOST deletes the session and clears the cookie.
// Uses POST (not GET) because logout changes server state.
func LogoutPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.DeleteSession(w, r, db); err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
