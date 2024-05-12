package main

import (
	"net/http"

	"github.com/gtsteffaniak/hi-time-live/routes"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	routes.SetupRoutes(e)
	// attempt tls first, fallback to unsecure
	if err := e.StartTLS(":9012", "cert.pem", "key.pem"); err != http.ErrServerClosed {
		e.Logger.Fatal(e.Start(":9012"))
	}
}
