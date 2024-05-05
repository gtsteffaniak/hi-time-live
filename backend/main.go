package main

import (
	"github.com/gtsteffaniak/hi-time-live/signal"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.Static("/", "frontend")
	e.PUT("/api/channel/create", signal.Create)
	e.PUT("/api/channel/join", signal.Join)
	e.Logger.Fatal(e.Start(":9012"))
}
