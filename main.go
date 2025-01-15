package main

import (
	"embed"
	"flag"

	"github.com/gtsteffaniak/hi-time-live/routes"
)

//go:embed static/*
var staticAssets embed.FS

//go:embed templates/*
var templateAssets embed.FS

func main() {
	devMode := flag.Bool("dev", false, "enable dev mode (hot-reloading and debug logging)")
	port := flag.Int("port", 0, "port to run program on")

	flag.Parse()
	routes.StartRouter(*devMode, *port, staticAssets, templateAssets)
}
