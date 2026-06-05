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
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		postID, err := strconv.ParseInt(r.FormValue("post_id"), 10, 64)
		if err != nil || postID <= 0 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		if err := database.DeletePost(db, postID); err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// DeleteAnyCommentPOST allows moderators and admins to delete any comment.
func DeleteAnyCommentPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		commentID, err := strconv.ParseInt(r.FormValue("comment_id"), 10, 64)
		if err != nil || commentID <= 0 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		postID, err := strconv.ParseInt(r.FormValue("post_id"), 10, 64)
		if err != nil || postID <= 0 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		if err := database.DeleteAnyComment(db, commentID); err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/post?id=%d", postID), http.StatusSeeOther)
	}
}

// AdminPanelGET shows the admin panel with all users and site stats.
func AdminPanelGET(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := database.GetAllUsers(db)
		if err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		stats := database.GetStats(db)
		userID, _ := utils.GetUserID(r)
		utils.RenderTemplate(w, "web/templates/admin.html", models.Template{
			Users:    users,
			LoggedIn: true,
			Role:     "admin",
			UserID:   userID,
			Username: getUsernameByID(db, userID),
			Stats:    stats,
		})
	}
}

// SetRolePOST allows admins to change a user's role.
func SetRolePOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		targetUserID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
		if err != nil || targetUserID <= 0 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		role := strings.TrimSpace(r.FormValue("role"))

		// Prevent admin from demoting themselves accidentally.
		currentUserID, _ := utils.GetUserID(r)
		if targetUserID == currentUserID {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		if err := database.SetUserRole(db, targetUserID, role); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}
