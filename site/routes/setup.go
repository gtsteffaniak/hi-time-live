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

// https://flowbite.com/icons/ - used for icons
// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templates *template.Template
}

func SetupRoutes(e *echo.Echo) {
	e.Static("/", "frontend")
	setupMiddleware(e)
	tempfiles, err := FindHTMLFiles("templates")
	if err != nil {
		panic("could not load template files!")
	}
	// Register custom template renderer
	renderer := &TemplateRenderer{
		template.Must(template.ParseFiles(tempfiles...)),
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
	data["code"] = uuid.New().String()
	return e.Render(200, "main.html", data)
}

func roomHandler(e echo.Context) error {
	id := e.QueryParam("id")
	data := map[string]interface{}{}
	data["code"] = id
	if !validCode(id) {
		return e.Render(200, "invalidRoom.html", data)
	} else {
		return e.Render(200, "room.html", data)
	}
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
