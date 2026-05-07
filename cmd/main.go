// cmd/main.go
// Entry point of the forum application.
// Wires together the database, routes, and HTTP server.

package main

import (
	"forum/internal/database"
	"forum/internal/handlers"
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

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlers.LogoutPOST(db)(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Forum is alive. Database connected"))
	})

	log.Println("Server starting on http://localhost:8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}

}
