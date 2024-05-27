package routes

import (
	"context"
	"html/template"
	"io"
	"log/slog"
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
	devMode     bool
}

func SetupRoutes(e *echo.Echo, devMode bool, logger slog.Logger) {
	e.Static("/", "static")
	setupMiddleware(e, logger)
	// Register custom template renderer
	t := &TemplateRenderer{
		templateDir: "templates",
		devMode:     devMode,
	}
	if err := t.loadTemplates(); err != nil {
		e.Logger.Fatal(err)
	}
	e.Renderer = t
	e.GET("/", indexHandler)
	e.GET("/room", roomHandler)
	e.GET("/ws", wsHandler)
}

func setupMiddleware(e *echo.Echo, logger slog.Logger) {
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		HandleError: true, // forwards error to the global error handler
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			level := slog.LevelInfo
			errMessage := ""
			if v.Error != nil {
				level = slog.LevelError
				errMessage = v.Error.Error()
			}
			logger.LogAttrs(context.Background(), level, "REQUEST",
				slog.Int("status", v.Status),
				slog.String("ip", v.RemoteIP),
				slog.String("referrer", v.Referer),
				slog.String("method", v.Method),
				slog.String("latency", v.Latency.String()),
				slog.String("uri", v.URI),
				slog.String("error", errMessage),
			)
			return nil
		},
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
			return err
		}
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

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	if t.devMode {
		if err := t.loadTemplates(); err != nil {
			slog.Error(err)
		}
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
	return t.templates.ExecuteTemplate(w, name, data)
}
