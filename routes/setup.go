package routes

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// BasePath is the URL prefix the app is served under (e.g. "/hitime" when
// reverse-proxied at https://example.com/hitime/). Empty means the root.
var BasePath string

// NormalizeBasePath returns p with a single leading slash and no trailing
// slash; "" and "/" both mean the root and normalize to "".
func NormalizeBasePath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// NewRouter builds the application's routes. Tests serve it directly.
func NewRouter() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("GET /events", sseHandler)       // Server-Sent Events endpoint
	router.HandleFunc("POST /event", postEventHandler) // Rest endpoint for client event responses
	router.HandleFunc("GET /room", roomHandler)
	router.HandleFunc("GET /", staticHandler)
	return router
}

// MountRouter serves router under basePath. Requests outside the prefix get a
// 404, and the bare prefix redirects to the trailing-slash form so relative
// URLs in the pages resolve correctly.
func MountRouter(router http.Handler, basePath string) http.Handler {
	basePath = NormalizeBasePath(basePath)
	if basePath == "" {
		return router
	}
	outer := http.NewServeMux()
	outer.Handle(basePath+"/", http.StripPrefix(basePath, router))
	outer.HandleFunc(basePath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, basePath+"/", http.StatusMovedPermanently)
	})
	return outer
}

func StartRouter(devMode bool, port int, basePath string) {
	BasePath = NormalizeBasePath(basePath)
	if BasePath != "" {
		log.Printf("Serving under base path: %s/", BasePath)
	}
	router := MountRouter(NewRouter(), BasePath)
	// Register custom template renderer
	templateRenderer = &TemplateRenderer{
		templateDir: "templates",
		devMode:     devMode,
	}
	err := templateRenderer.loadTemplates()
	if err != nil {
		log.Fatalf("could not load templates: %v", err)
	}

	// Attempt to load the TLS certificate and key
	cer, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Printf("could not load certificate, falling back to HTTP: %v", err)

		// Fallback to HTTP on port 80
		if port == 0 {
			port = 80
		}

		fullURL := fmt.Sprintf("http://localhost:%d", port)
		log.Printf("Running in HTTP mode at: %s", fullURL)
		err = http.ListenAndServe(fmt.Sprintf(":%d", port), muxWithMiddleware(router))
		if err != nil {
			log.Fatalf("could not start HTTP server: %v", err)
		}
		return
	}

	// Create a custom TLS listener
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cer},
	}

	// Set HTTPS scheme and default port for TLS
	scheme := "https"
	if port == 0 {
		port = 443
	}

	// Listen on TCP and wrap with TLS
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%v", port), tlsConfig)
	if err != nil {
		log.Fatalf("could not start TLS server: %v", err)
	}
	// Build the full URL with host and port
	fullURL := fmt.Sprintf("%v://localhost:%v", scheme, port)
	log.Printf("Running at               : %s", fullURL)
	err = http.Serve(listener, muxWithMiddleware(router))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
