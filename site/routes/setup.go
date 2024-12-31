package routes

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
)

func StartRouter(devMode bool, logger slog.Logger) {
	router := http.NewServeMux()
	// API group routing
	api := http.NewServeMux()

	api.HandleFunc("GET /room", roomHandler)
	router.Handle("/api", http.StripPrefix("/api", api))
	router.HandleFunc("GET /static/", indexHandler)
	router.HandleFunc("/", indexHandler)
	// Register custom template renderer
	templateRenderer = &TemplateRenderer{
		templateDir: "templates",
		devMode:     devMode,
	}
	templateRenderer.loadTemplates()
	slog.Info("listening: http://localhost")
	err := http.ListenAndServe(fmt.Sprintf(":%v", 80), muxWithMiddleware(router))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}

}
