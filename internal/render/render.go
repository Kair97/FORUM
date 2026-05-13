package render

import (
	"forum/internal/models"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
)

// Error renders the shared error page with the correct HTTP status code.
// It lives in its own package so handlers, utils, and middleware can all use it
// without creating an import cycle.
func Error(w http.ResponseWriter, status int) {
	code := strconv.Itoa(status)

	w.WriteHeader(status)

	tmpl, err := template.ParseFiles(
		"web/templates/base.html",
		"web/templates/error.html",
	)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, filepath.Base("web/templates/base.html"), models.Template{
		Error: code,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
