package utils

import (
	"forum/internal/middleware"
	"net/http"
	"path/filepath"
	"text/template"
)

// Utils - utility functions
// Small reusable helper functions that don’t belong to a specific feature.

// GetUserID reads the authenticated user's ID from the request context.
// Returns the userID and true if the user is logged in.
// Returns 0 and false if the user is not logged in.
// Handlers use this to check auth status and get the current user's ID
func GetUserID(r *http.Request) (int64, bool) {
	// r.Context().Value() returns interface {} - we must type assert it.
	// The comma-ok pattern: if the key does not exist or the type is wrong,
	// ok will be false instead of panicking.
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	return userID, ok
}

// RenderTemplate parses and executes an HTML template.
// It always includes base.html as the layout wrapper.
// pagePath is the path to the specific page template.
// data is passed directly to the template engine.
func RenderTemplate(w http.ResponseWriter, pagePath string, data interface{}) {
	// We parse base.html + the specific page template together.
	// The base defines the outer HTML structure.
	// The page defines the "content" block that goes inside base.
	tmpl, err := template.ParseFiles(
		"web/templates/base.html",
		pagePath,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute runs the template with the provided data and writes
	// the result directly to the HTTP response writer.
	// "base.html" refers to the {{define "base"}} block in base.html —
	// we execute the base which then pulls in the content block.
	err = tmpl.ExecuteTemplate(w, filepath.Base("web/templates/base.html"), data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

}
