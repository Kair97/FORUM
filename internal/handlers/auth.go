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
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "All fields are required",
			})

			return
		}

		if len(password) < 6 {
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "Password is too short!",
			})

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
		utils.RenderTemplate(w, login, models.Template{})
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
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, login, models.Template{
				Error: "All fields are required!",
			})
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
			w.WriteHeader(http.StatusUnauthorized)
			utils.RenderTemplate(w, login, models.Template{
				Error: "Invalid email or password",
			})
			return
		}

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Verify the password
		if err := auth.CheckPassword(password, user.PasswordHash); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			utils.RenderTemplate(w, login, models.Template{
				Error: "Invalid email or password",
			})
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
