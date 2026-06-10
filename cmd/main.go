package main

import (
	"context"
	"database/sql"
	"forum/internal/database"
	"forum/internal/handlers"
	"forum/internal/middleware"
	"forum/internal/render"
	"forum/internal/utils"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. Open database and run migrations
	db, err := database.Init("./volume/database/forum.db", "./migrations/schema.sql")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// 2. Parse all HTML templates once at startup
	render.Init()

	// 3. Register all routes on one mux
	mux := http.NewServeMux()
	registerRoutes(mux, db)

	// 4. Create the server
	server := &http.Server{
		Addr:    ":8080",
		Handler: middleware.SecurityHeaders(mux),
	}

	// 5. Start server in background, then wait for a shutdown signal
	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server error:", err)
		}
	}()

	// Block until we receive Ctrl+C (SIGINT) or a kill signal (SIGTERM).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Give active requests up to 10 seconds to finish before forcing a stop.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server stopped gracefully")
}

// registerRoutes wires every URL path to its handler on the given mux.
func registerRoutes(mux *http.ServeMux, db *sql.DB) {
	// Static assets: CSS, favicon, etc.
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// User-uploaded images — served from the volume directory, not web/static.
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("volume/uploaded_imgs"))))

	// ----------- Auth routes -----------------------------------------------

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.RegisterGET(db)(w, r)
		case http.MethodPost:
			handlers.RegisterPOST(db)(w, r)
		default:
			utils.RenderError(w, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.LoginGET(db)(w, r)
		case http.MethodPost:
			handlers.LoginPOST(db)(w, r)
		default:
			utils.RenderError(w, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		handlers.LogoutPOST(db)(w, r)
	})

	// ----------- Post routes -----------------------------------------------

	// Homepage — guests can view, logged-in users see extra controls.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		middleware.OptionalAuth(handlers.IndexGET(db), db)(w, r)
	})

	// Single post page.
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.OptionalAuth(handlers.PostGET(db), db)(w, r)
	})

	// Create post — requires login.
	mux.HandleFunc("/post/create", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAuth(handlers.CreatePostGET(db), db)(w, r)
		case http.MethodPost:
			middleware.RequireAuth(handlers.CreatePostPOST(db), db)(w, r)
		default:
			utils.RenderError(w, http.StatusMethodNotAllowed)
		}
	})

	// ----------- Comment routes --------------------------------------------

	mux.HandleFunc("/comment/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAuth(handlers.CreateCommentPOST(db), db)(w, r)
	})

	mux.HandleFunc("/comment/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAuth(handlers.DeleteCommentPOST(db), db)(w, r)
	})

	// ----------- Like route ------------------------------------------------

	mux.HandleFunc("/like", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAuth(handlers.ToggleLikePOST(db), db)(w, r)
	})

	// ----------- Moderation routes -----------------------------------------

	mux.HandleFunc("/mod/delete-post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireModerator(handlers.DeletePostPOST(db), db)(w, r)
	})

	mux.HandleFunc("/mod/delete-comment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireModerator(handlers.DeleteAnyCommentPOST(db), db)(w, r)
	})

	// ----------- Admin routes ----------------------------------------------

	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAdmin(handlers.AdminPanelGET(db), db)(w, r)
	})

	mux.HandleFunc("/admin/set-role", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.RenderError(w, http.StatusMethodNotAllowed)
			return
		}
		middleware.RequireAdmin(handlers.SetRolePOST(db), db)(w, r)
	})
}
