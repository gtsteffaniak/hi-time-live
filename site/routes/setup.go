package routes

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templateDir string
	templates   *template.Template
}

func SetupRoutes(e *echo.Echo) {
	e.Static("/", "static")
	setupMiddleware(e)
	// Register custom template renderer
	renderer := &TemplateRenderer{
		templateDir: "templates",
	}
	if err := renderer.loadTemplates(); err != nil {
		e.Logger.Fatal(err)
	}
	e.Renderer = renderer
	e.GET("/", indexHandler)
	e.GET("/room", roomHandler)
	e.GET("/ws", wsHandler)
}

func setupMiddleware(e *echo.Echo) {
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		CustomTimeFormat: "2006-01-02 15:04:05.00",
		Format:           "[HTTP] ${time_custom} | ${status} | ${remote_ip} :${referrer} | ${method} | ${latency_human}  | \"${uri}\"\n",
		Output:           e.Logger.Output(),
	}))
	e.Use(middleware.Secure())
	e.Use(middleware.Recover())
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

func indexHandler(c echo.Context) error {
	data := map[string]interface{}{}
	id := uuid.New().String()
	data["code"] = id
	data["joinModal"] = map[string]string{
		"modalType": "join",
		"hidden":    "hidden",
		"code":      id,
	}
	return c.Render(200, "main.html", data)
}

func roomHandler(c echo.Context) error {
	id := c.QueryParam("id")
	data := map[string]interface{}{}
	data["code"] = id
	data["privacyModal"] = map[string]string{
		"modalType": "privacy",
		"hidden":    "",
		"code":      id,
	}
	if !validCode(id) {
		return c.Render(200, "invalidRoom.html", data)
	} else {
		return c.Render(200, "room.html", data)
	}
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
			return fmt.Errorf("error reading file: %w", err)
		}
		file = strings.TrimPrefix(file, t.templateDir+"/")
		fmt.Println("processing ", file)
		fileContent := string(content)
		_, err = t.templates.New(file).Parse(fileContent)
		if err != nil {
			return err
		}
	}
	return nil
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	noCacheHeaders := map[string]string{
		"Cache-Control":     "no-cache, private, max-age=0",
		"Pragma":            "no-cache",
		"X-Accel-Expires":   "0",
		"Transfer-Encoding": "identity",
	}
	for k, v := range noCacheHeaders {
		c.Response().Header().Set(k, v)
	}

	return t.templates.ExecuteTemplate(w, name, data)
}
