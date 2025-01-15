package routes

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"io/fs"
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
	staticFileServer http.Handler
)

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templateDir    string
	templates      *template.Template
	devMode        bool
	templateAssets embed.FS
	staticAssets   embed.FS
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

// GetEmbededTemplates returns a list of template file paths from the embedded filesystem
func (t *TemplateRenderer) GetEmbededTemplates() ([]string, error) {
	var files []string
	err := fs.WalkDir(t.templateAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (t *TemplateRenderer) loadTemplates() error {
	var tempfiles []string
	var err error

	if t.devMode {
		tempfiles, err = FindFiles(t.templateDir)
		if err != nil {
			return err
		}
	} else {
		staticFileServer = http.FileServer(http.FS(templateRenderer.staticAssets))

		tempfiles, err = t.GetEmbededTemplates()
		if err != nil {
			return err
		}
	}

	t.templates = template.New("")

	for _, file := range tempfiles {
		var content []byte
		if t.devMode {
			content, err = os.ReadFile(file)
			if err != nil {
				return err
			}
		} else {
			content, err = t.templateAssets.ReadFile(file)
			if err != nil {
				return err
			}
		}
		file = strings.ReplaceAll(file, "\\", "/")
		file = strings.TrimPrefix(file, t.templateDir+"/")
		//log.Println("processing file: " + file)
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
	staticPath := "static" + r.URL.Path
	if templateRenderer.devMode {
		http.ServeFile(w, r, staticPath)
	} else {
		serveFromEmbeddedFS(w, r, staticPath)
	}
}

func serveFromEmbeddedFS(w http.ResponseWriter, r *http.Request, staticPath string) {
	// Attempt to open the file from the embedded filesystem
	file, err := templateRenderer.staticAssets.Open(staticPath)
	if err != nil {
		// File not found in the embedded filesystem
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Get file info (e.g., for ModTime)
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Read the file content into memory
	data, err := io.ReadAll(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Serve the content
	http.ServeContent(w, r, staticPath, info.ModTime(), bytes.NewReader(data))
}
