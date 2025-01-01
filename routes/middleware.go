package routes

import (
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func muxWithMiddleware(mux *http.ServeMux) *http.ServeMux {
	wrappedMux := http.NewServeMux()
	wrappedMux.Handle("/", LoggingMiddleware(mux))
	return wrappedMux
}

// ResponseWriterWrapper wraps the standard http.ResponseWriter to capture the status code
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode  int
	wroteHeader bool
	PayloadSize int
}

// WriteHeader captures the status code and ensures it's only written once
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	if !w.wroteHeader { // Prevent WriteHeader from being called multiple times
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		w.StatusCode = statusCode
		w.ResponseWriter.WriteHeader(statusCode)
		w.wroteHeader = true
	}
}

// LoggingMiddleware logs each request and its status code.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		// Wrap the ResponseWriter to capture the status code.
		wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		// Call the next handler.
		next.ServeHTTP(wrappedWriter, r)

		// Determine the color based on the status code.
		color := "\033[32m" // Default green color
		if wrappedWriter.StatusCode >= 300 && wrappedWriter.StatusCode < 500 {
			color = "\033[33m" // Yellow for client errors (4xx)
		} else if wrappedWriter.StatusCode >= 500 {
			color = "\033[31m" // Red for server errors (5xx)
		}

		// Capture the full URL path including the query parameters.
		fullURL := r.URL.Path
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}

		// Log the request, status code, and response size.
		log.Printf("%s%-7s | %3d | %-15s | %-12s | \"%s\"%s",
			color,
			r.Method,
			wrappedWriter.StatusCode, // Captured status code
			r.RemoteAddr,
			time.Since(start).String(),
			fullURL,
			"\033[0m", // Reset color
		)
	})
}

func renderJSON(w http.ResponseWriter, r *http.Request, data interface{}) (int, error) {
	marsh, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	// Calculate size in KB
	payloadSizeKB := len(marsh) / 1024
	// Check if the client accepts gzip encoding and hasn't explicitly disabled it
	if acceptsGzip(r) && payloadSizeKB > 10 {
		// Enable gzip compression
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()

		if _, err := gz.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		// Normal response without compression
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := w.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	return 0, nil
}

func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return ae != "" && strings.Contains(ae, "gzip")
}
