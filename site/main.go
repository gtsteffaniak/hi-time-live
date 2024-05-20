package main

import (
	"log"
	"net/http"

	"github.com/gtsteffaniak/hi-time-live/routes"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	routes.SetupRoutes(e)
	// attempt tls first, fallback to unsecure
	if err := e.StartTLS(":9012", "cert.pem", "key.pem"); err != http.ErrServerClosed {
		log.Println("WARNING: The server is running with HTTP instead of HTTPS, please ensure its accessed behind a reverse proxy with HTTPS.")
		e.Logger.Fatal(e.Start(":9012"))
	}
}
