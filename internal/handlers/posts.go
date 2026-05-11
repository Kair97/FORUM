// internal/handlers/posts.go
// Handles HTTP requests for viewing and creating posts.

package handlers

import (
	"database/sql"
	"fmt"
	"forum/internal/database"
	"forum/internal/middleware"
	"forum/internal/models"
	"forum/internal/utils"
	"net/http"
	"strconv"
	"strings"
)

const index = "web/templates/index.html"
const postpage = "web/templates/post.html"
const cpost = "web/templates/create-post.html"

// IndexGET serves the homepage showing all posts.
// Available to all users - guests and logged-in users.

// IndexGET now serves the homepage with optional filtering.
// Filter logic:
//
//	/?category_id=N  → posts in that category  (all users)
//	/?filter=created → posts by logged-in user  (auth required)
//	/?filter=liked   → posts liked by user      (auth required)
//	/                → all posts                (all users)
func IndexGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The default ServeMux routes everything that does not match
		// another pattern to "/". We only want to handle exactly "/".
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Read the filter parameters from the URL
		categoryIDStr := r.URL.Query().Get("category_id")
		filter := r.URL.Query().Get("filter")

		// Read login status — needed for auth-only filters.
		userID, loggedIn := utils.GetUserID(r)

		categories, err := database.GetAllCategories(db)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var posts []models.Post
		var categoryID int64

		switch {
		case categoryIDStr != "":
			// Filter by category - available to all users.
			categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid Category ID", http.StatusBadRequest)
				return
			}

			posts, err = database.GetPostsByCategory(db, categoryID, userID)
			if err != nil {
				http.Error(w, "innternal Server Error", http.StatusInternalServerError)
				return
			}
		case filter == "created":
			// Filter by posts created by the logged-in user.
			// Guests are redirected to login.
			if !loggedIn {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			posts, err = database.GetPostsByUser(db, userID)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		case filter == "liked":
			// Filter by posts liked by the logged-in user.
			// Guests are redirected to login.
			if !loggedIn {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			posts, err = database.GetPostsLikedByUser(db, userID)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		default:
			// No filter so all posts are showed
			posts, err = database.GetAllPosts(db, userID)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return

			}
		}

		var username string
		if loggedIn {
			username = getUsernameByID(db, userID)
		}

		utils.RenderTemplate(w, index, models.Template{
			Posts:      posts,
			Categories: categories,
			LoggedIn:   loggedIn,
			UserID:     userID,
			Username:   username,
			Filter:     filter,
			CategoryID: categoryID,
		})

	}
}

func getUsernameByID(db *sql.DB, userID int64) string {
	var username string
	err := db.QueryRow(
		"SELECT username FROM users WHERE id = ?", userID,
	).Scan(&username)

	if err != nil {
		return ""
	}
	return username
}

// PostGET serves the single post page with all its comments.
// Available to all users.
func PostGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the post ID from the URL query parameter.
		// URL format: /post?id=5
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "Missing Post ID", http.StatusBadRequest)
			return
		}

		// strconv.ParseInt converts string to integer "5" ==> 5
		// Base 10, 64-bit integer.
		postID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid Post ID", http.StatusBadRequest)
			return
		}

		userID, _ := utils.GetUserID(r)
		// Fetch the post from database
		post, comments, err := database.GetPostByID(db, postID, userID)
		if err == sql.ErrNoRows {
			// Post not found - return 404
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		userID, loggedIn := utils.GetUserID(r)
		var username string
		username = getUsernameByID(db, userID)

		utils.RenderTemplate(w, postpage, models.Template{
			Post:     post,
			Comments: comments,
			LoggedIn: loggedIn,
			UserID:   userID,
			Username: username,
		})
	}
}

// CreatePostGET serves the create post form.
// Requires authentication — middleware handles the redirect if not logged in.
func CreatePostGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categories, err := database.GetAllCategories(db)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		userID, _ := utils.GetUserID(r)

		utils.RenderTemplate(w, cpost, models.Template{
			Categories: categories,
			LoggedIn:   true,
			Username:   getUsernameByID(db, userID),
		})

	}
}

// CreatePostPOST processes the new post form submission.
// Requires authentication.
func CreatePostPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, _ := utils.GetUserID(r)

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		title := strings.TrimSpace(r.FormValue("title"))
		content := strings.TrimSpace(r.FormValue("content"))

		// Validate: title and content must not be empty
		if title == "" || content == "" {
			http.Error(w, "Title and content are required", http.StatusBadRequest)
			return
		}

		// Read the selected category IDs from the form.
		// The form will send multiple values for "category_ids"
		// r.Form["category_ids"] returns []string of all selected values.
		categoryStrs := r.Form["category_ids"]
		if len(categoryStrs) == 0 {
			http.Error(w, "Select at least one category", http.StatusBadRequest)
			return
		}

		// Convert []string to []int64
		var categoryIDs []int64
		for _, s := range categoryStrs {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				http.Error(w, "Invalid category ID", http.StatusBadRequest)
				return
			}
			categoryIDs = append(categoryIDs, id)
		}

		// Create the post in the database.
		// For now we will not add image path
		postID, err := database.CreatePost(db, userID, title, content, "", categoryIDs)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Redirect to the new post's page.
		http.Redirect(w, r, fmt.Sprintf("/post?id=%d", postID), http.StatusSeeOther)

	}
}

var _ = middleware.UserIDKey
