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
			utils.RenderError(w, http.StatusBadRequest)
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

		// Length limits — prevent database abuse and UI breakage.
		if len(email) > 254 {
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "Email must be 254 characters or less",
			})
			return
		}
		if len(username) > 30 {
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "Username must be 30 characters or less",
			})
			return
		}
		if len(password) > 72 {
			// bcrypt silently truncates passwords over 72 bytes.
			// We reject them explicitly so the user knows the limit.
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "Password must be 72 characters or less",
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
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "Email already registered",
			})
			return
		}
		if err != sql.ErrNoRows {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		// Check whether username is already used.
		// The database also has UNIQUE(username), but checking here lets us
		// show a clear registration error instead of a generic server error.
		err = db.QueryRow(
			"SELECT id FROM users WHERE username = ?", username,
		).Scan(&existingID)

		if err == nil {
			w.WriteHeader(http.StatusBadRequest)
			utils.RenderTemplate(w, register, models.Template{
				Error: "Username already taken",
			})
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

		// Insert new user to database
		result, err := db.Exec(
			"INSERT INTO users (email, username, password_hash, role) VALUES (?, ?, ?, ?)", email, username, passwordHash, "user",
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

		ip := r.RemoteAddr
		if i := strings.LastIndex(ip, ":"); i != -1 {
			ip = ip[:i]
		}

		if !utils.LoginLimiter.Allow(ip) {
			utils.RenderTemplate(w, login, models.Template{
				Error: "Too many login attempts. Please wait a minute and try again.",
			})
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
			utils.RenderError(w, http.StatusInternalServerError)
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
			utils.RenderError(w, http.StatusInternalServerError)
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
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		// Redirect to homepage after logout
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
