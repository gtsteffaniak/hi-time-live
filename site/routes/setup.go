package routes

import (
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templateDir string
}

func SetupRoutes(e *echo.Echo) {
	e.Static("/", "static")
	setupMiddleware(e)
	// Register custom template renderer
	renderer := &TemplateRenderer{
		templateDir: "templates",
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

func FindHTMLFiles(rootPath string) ([]string, error) {
	var htmlFiles []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".html" {
			htmlFiles = append(htmlFiles, path)
		}

		return nil
	})

	return htmlFiles, err
}
func indexHandler(e echo.Context) error {
	data := map[string]interface{}{}
	id := uuid.New().String()
	data["code"] = id
	data["joinModal"] = map[string]string{
		"modalType": "join",
		"hidden":    "hidden",
		"code":      id,
	}
	return e.Render(200, "main.html", data)
}

func roomHandler(e echo.Context) error {
	id := e.QueryParam("id")
	data := map[string]interface{}{}
	data["code"] = id
	data["privacyModal"] = map[string]string{
		"modalType": "privacy",
		"hidden":    "",
		"code":      id,
	}
	if !validCode(id) {
		return e.Render(200, "invalidRoom.html", data)
	} else {
		return e.Render(200, "room.html", data)
	}
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {

	tempfiles, err := FindHTMLFiles(t.templateDir)
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFiles(tempfiles...)
	if err != nil {
		return err
	}

	noCacheHeaders := map[string]string{
		"Cache-Control":     "no-cache, private, max-age=0",
		"Pragma":            "no-cache",
		"X-Accel-Expires":   "0",
		"Transfer-Encoding": "identity",
	}
	for k, v := range noCacheHeaders {
		c.Response().Header().Set(k, v)
	}
	return tmpl.ExecuteTemplate(w, name, data)
}
