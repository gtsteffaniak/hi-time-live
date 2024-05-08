package main

import (
	"github.com/gtsteffaniak/hi-time-live/routes"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	routes.SetupRoutes(e)
	e.Logger.Fatal(e.Start(":9012"))
}
