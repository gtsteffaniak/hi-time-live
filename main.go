package main

import (
	"flag"

	"github.com/gtsteffaniak/hi-time-live/routes"
)

func main() {
	devMode := flag.Bool("dev", false, "enable dev mode (hot-reloading and debug logging)")
	flag.Parse()
	routes.StartRouter(*devMode)
}
