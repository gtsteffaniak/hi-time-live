package main

import (
	"flag"
	"os"

	"github.com/gtsteffaniak/hi-time-live/routes"
)

func main() {
	devMode := flag.Bool("dev", false, "enable dev mode (hot-reloading and debug logging)")
	port := flag.Int("port", 0, "port to run program on")
	basePath := flag.String("base-path", os.Getenv("BASE_PATH"), "URL prefix to serve under, e.g. /hitime (default: $BASE_PATH)")

	flag.Parse()
	routes.StartRouter(*devMode, *port, *basePath)
}
