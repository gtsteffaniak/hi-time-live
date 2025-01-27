package routes

import (
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var (
	templateRenderer *TemplateRenderer
	Version          string
)

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templateDir string
	templates   *template.Template
	devMode     bool
}

// Render renders a template document with headers and data
func (t *TemplateRenderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	if t.devMode {
		if err := t.loadTemplates(); err != nil {
			slog.Error("unable to parse templates", "error", err)
		}
	}
	// Set headers
	w.Header().Set("Cache-Control", "no-cache, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Accel-Expires", "0")
	w.Header().Set("Transfer-Encoding", "identity")
	// Execute the template with the provided data
	return t.templates.ExecuteTemplate(w, name, data)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{}
	id := uuid.New().String()
	fmt.Println("version", Version)
	data["version"] = Version
	data["code"] = id
	data["joinModal"] = map[string]string{
		"modalType": "join",
		"hidden":    "hidden",
		"code":      id,
	}
	err := templateRenderer.Render(w, "main.html", data)
	if err != nil {
		log.Println("could not render main.html template", http.StatusInternalServerError)
	}
}

func FindFiles(rootPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (t *TemplateRenderer) loadTemplates() error {
	tempfiles, err := FindFiles(t.templateDir)
	if err != nil {
		return err
	}
	t.templates = template.New("")
	for _, file := range tempfiles {
		// Read the file content
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		file = strings.ReplaceAll(file, "\\", "/")
		file = strings.TrimPrefix(file, t.templateDir+"/")
		slog.Debug("processing file: " + file)
		fileContent := string(content)
		_, err = t.templates.New(file).Parse(fileContent)
		if err != nil {
			return err
		}
	}
	return nil
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		indexHandler(w, r)
		return
	}
	fmt.Println("static handler", r.URL.Path)
	http.ServeFile(w, r, "static"+r.URL.Path)
}
