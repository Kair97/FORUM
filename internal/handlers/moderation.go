// internal/handlers/moderation.go
// Handles moderator and admin actions.

package handlers

import (
	"database/sql"
	"fmt"
	"forum/internal/database"
	"forum/internal/models"
	"forum/internal/utils"
	"net/http"
	"strconv"
	"strings"
)

// DeletePostPOST allows moderators and admins to delete any post.
func DeletePostPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		postID, err := strconv.ParseInt(r.FormValue("post_id"), 10, 64)
		if err != nil || postID <= 0 {
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		if err := database.DeletePost(db, postID); err != nil {
			http.Error(w, "Could not delete post", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// DeleteAnyCommentPOST allows moderators and admins to delete any comment.
func DeleteAnyCommentPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		commentID, err := strconv.ParseInt(r.FormValue("comment_id"), 10, 64)
		if err != nil || commentID <= 0 {
			http.Error(w, "Invalid comment ID", http.StatusBadRequest)
			return
		}

		postID, err := strconv.ParseInt(r.FormValue("post_id"), 10, 64)
		if err != nil || postID <= 0 {
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		if err := database.DeleteAnyComment(db, commentID); err != nil {
			http.Error(w, "Could not delete comment", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/post?id=%d", postID), http.StatusSeeOther)
	}
}

// AdminPanelGET shows the admin panel with all users.
func AdminPanelGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := database.GetAllUsers(db)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		userID, _ := utils.GetUserID(r)
		utils.RenderTemplate(w, "web/templates/admin.html", models.Template{
			Users:    users,
			LoggedIn: true,
			Role:     "admin",
			Username: getUsernameByID(db, userID),
		})
	}
}

// SetRolePOST allows admins to change a user's role.
func SetRolePOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		targetUserID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
		if err != nil || targetUserID <= 0 {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		role := strings.TrimSpace(r.FormValue("role"))

		// Prevent admin from demoting themselves accidentally.
		currentUserID, _ := utils.GetUserID(r)
		if targetUserID == currentUserID {
			http.Error(w, "Cannot change your own role", http.StatusBadRequest)
			return
		}

		if err := database.SetUserRole(db, targetUserID, role); err != nil {
			http.Error(w, "Failed to update role", http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}
