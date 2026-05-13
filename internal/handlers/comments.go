// internal/handlers/comments.go
// Handles HTTP requests for creating comments on posts.

package handlers

import (
	"database/sql"
	"fmt"
	"forum/internal/database"
	"forum/internal/utils"
	"net/http"
	"strconv"
	"strings"
)

// CreateCommentPOST processes a new comment form submission.
// Requires authentication - middlware handles redirect if user not logged in.
func CreateCommentPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the ID of logged in user
		userID, _ := utils.GetUserID(r)

		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		postIDStr := strings.TrimSpace(r.FormValue("post_id"))
		content := strings.TrimSpace(r.FormValue("content"))

		postID, err := strconv.ParseInt(postIDStr, 10, 64)
		if err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		// Validate comment must not be empty
		if content == "" {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		if len(content) > 2000 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		// Insert the comment into the database
		_, err = database.CreateComment(db, postID, userID, content)
		if err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/post?id=%d", postID), http.StatusSeeOther)
	}
}

func DeleteCommentPOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, _ := utils.GetUserID(r)

		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		commentIDStr := r.FormValue("comment_id")
		postIDStr := r.FormValue("post_id")

		commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
		if err != nil || commentID <= 0 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		postID, err := strconv.ParseInt(postIDStr, 10, 64)
		if err != nil || postID <= 0 {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		if err := database.DeleteComment(db, commentID, userID); err != nil {
			utils.RenderError(w, http.StatusForbidden)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/post?id=%d", postID), http.StatusSeeOther)

	}
}
