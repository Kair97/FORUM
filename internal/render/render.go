package render

import (
	"forum/internal/models"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
)

// templates holds every parsed page template, keyed by filename (e.g. "index.html").
// It is written once in Init and then only read — so concurrent requests are safe, no data race.
var templates map[string]*template.Template

// Init parses every HTML template exactly once when the app starts.
// If any template file is missing or broken, the app exits immediately (fail fast).
func Init() {
	pages := []string{
		"index.html",
		"post.html",
		"create-post.html",
		"login.html",
		"register.html",
		"admin.html",
		"error.html",
	}

	templates = make(map[string]*template.Template, len(pages))

	for _, page := range pages {
		// Each page template is always combined with base.html (the layout wrapper).
		tmpl, err := template.ParseFiles(
			"web/templates/base.html",
			"web/templates/"+page,
		)
		if err != nil {
			log.Fatalf("failed to parse template %s: %v", page, err)
		}
		templates[page] = tmpl
	}
}

// Template executes a pre-parsed template by page path or filename.
// pagePath can be the full path ("web/templates/index.html") or just "index.html".
func Template(w http.ResponseWriter, pagePath string, data interface{}) {
	name := filepath.Base(pagePath) // "web/templates/index.html" → "index.html"
	tmpl, ok := templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	err := tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Error renders the error page with the given HTTP status code.
func Error(w http.ResponseWriter, status int) {
	code := strconv.Itoa(status)
	w.WriteHeader(status)

	tmpl, ok := templates["error.html"]
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err := tmpl.ExecuteTemplate(w, "base.html", models.Template{
		Error: code,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
