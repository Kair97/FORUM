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

// ToggleLikePOST processes a like or dislike action.
// Requires authentication.
// Accepts: target_type (post|comment), target_id, is_like (1|0)
func ToggleLikePOST(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the user is logged in.
		// If not, send them to the login page instead of silently failing.
		userID, loggedIn := utils.GetUserID(r)
		if !loggedIn {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if err := r.ParseForm(); err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		targetType := strings.TrimSpace(r.FormValue("target_type"))
		targetIDStr := strings.TrimSpace(r.FormValue("target_id"))
		isLikeStr := strings.TrimSpace(r.FormValue("is_like"))

		// Validate targetType: it should be exact "post" or "comment"
		if targetType != "post" && targetType != "comment" {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		// Parse and validate targetID.
		targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		// Parse isLikeStr it should be 1(true) or 0(false)
		if isLikeStr != "1" && isLikeStr != "0" {
			utils.RenderError(w, http.StatusBadRequest)
			return
		}

		isLike := isLikeStr == "1"

		if err := database.ToggleLike(db, userID, targetID, targetType, isLike); err != nil {
			utils.RenderError(w, http.StatusInternalServerError)
			return
		}

		// Redirect back to the referring page.
		// r.FormValue("redirect") lets the form tell us where go to back to.
		// If no redirect is specified, go to homepage
		redirect := r.FormValue("redirect")
		if redirect == "" {
			redirect = "/"
		}

		// Security: only allow redirects to our own paths.
		// This prevents open redirect attacks where an attacker could redirect
		// users to a malicious external site after liking.
		if !strings.HasPrefix(redirect, "/") {
			redirect = "/"
		}

		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}

var _ = fmt.Sprintf
