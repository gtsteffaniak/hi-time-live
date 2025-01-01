package routes

import (
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net/http"
)

func StartRouter(devMode bool, logger slog.Logger) {
	router := http.NewServeMux()
	router.HandleFunc("GET /room", roomHandler)
	//router.HandleFunc("GET /static/", indexHandler)
	router.HandleFunc("GET /events", sseHandler)
	router.HandleFunc("PUT /event", putEventHandler)
	router.HandleFunc("/", indexHandler)
	// Register custom template renderer
	templateRenderer = &TemplateRenderer{
		templateDir: "templates",
		devMode:     devMode,
	}
	templateRenderer.loadTemplates()

	// Load the TLS certificate and key
	cer, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatalf("could not load certificate: %v", err)
	}

	// Create a custom TLS listener
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cer},
	}

	// Set HTTPS scheme and default port for TLS
	scheme := "https"
	port := 443

	// Listen on TCP and wrap with TLS
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%v", port), tlsConfig)
	if err != nil {
		log.Fatalf("could not start TLS server: %v", err)
	}
	// Build the full URL with host and port
	fullURL := fmt.Sprintf("%v://localhost%v", scheme, port)
	log.Printf("Running at               : %s", fullURL)
	err = http.Serve(listener, muxWithMiddleware(router))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
	slog.Info("listening: http://localhost")
}