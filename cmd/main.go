// cmd/main.go
// Entry point of the forum application.
// Wires together the database, routes, and HTTP server.

package main

import (
	"fmt"
	"forum/internal/database"
	"forum/internal/handlers"
	"forum/internal/middleware"
	"forum/internal/utils"
	"log"
	"net/http"
)

func main() {

	db, err := database.Init("./internal/database/forum.db", "./migrations/schema.sql")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	defer db.Close()

	log.Println("Database initialized successfully")

	// Serve static files (CSS, JS, uploads) from web/static/.
	// http.StripPrefix removes "/static" from the URL before looking up the file.
	// So /static/css/style.css → web/static/css/style.css on disk.
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// ----------- Auth routes -----------------------------------------------
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.RegisterGET(db)(w, r)
		case http.MethodPost:
			handlers.RegisterPOST(db)(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.LoginGET(db)(w, r)
		case http.MethodPost:
			handlers.LoginPOST(db)(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/comment/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAuth(handlers.DeleteCommentPOST(db), db)(w, r)
	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlers.LogoutPOST(db)(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// ----------- Post routes -----------------------------------------------

	// Homepage — optional auth (guests can view)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		middleware.OptionalAuth(handlers.IndexGET(db), db)(w, r)
	})

	// Single post page - optional auth (guests can view)
	http.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		middleware.OptionalAuth(handlers.PostGET(db), db)(w, r)
	})

	// Create post - Require auth
	http.HandleFunc("/post/create", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAuth(handlers.CreatePostGET(db), db)(w, r)
		case http.MethodPost:
			middleware.RequireAuth(handlers.CreatePostPOST(db), db)(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Create comment - requires auth
	http.HandleFunc("/comment/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAuth(handlers.CreateCommentPOST(db), db)(w, r)
	})

	// Like/Dislike - requires auth
	http.HandleFunc("/like", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAuth(handlers.ToggleLikePOST(db), db)(w, r)
	})

	http.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
		middleware.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := utils.GetUserID(r)
			fmt.Fprintf(w, "You reached a protected route. Your user ID is: %d", userID)
		}, db)(w, r)
	})

	log.Println("Server starting on http://localhost:8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}

}
