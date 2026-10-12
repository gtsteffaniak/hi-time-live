package main

import (
	"flag"
	"log"
	"os"

	"github.com/gtsteffaniak/hi-time-live/routes"
)

func main() {
	devMode := flag.Bool("dev", false, "enable dev mode (hot-reloading and debug logging)")
	port := flag.Int("port", 0, "port to run program on")
	basePath := flag.String("base-path", os.Getenv("BASE_PATH"), "URL prefix to serve under, e.g. /hitime (default: $BASE_PATH)")
	iceServers := flag.String("ice-servers", os.Getenv("ICE_SERVERS"), "JSON array of RTCIceServer objects; dev-only override of the STUN/TURN flags (default: $ICE_SERVERS)")
	turnURL := flag.String("turn-url", os.Getenv("TURN_URL"), "TURN server URL(s), comma separated, e.g. turn:turn.example.com:3478; dev-only (default: $TURN_URL)")
	turnUsername := flag.String("turn-username", os.Getenv("TURN_USERNAME"), "TURN username; dev-only (default: $TURN_USERNAME)")
	turnCredential := flag.String("turn-credential", os.Getenv("TURN_CREDENTIAL"), "TURN credential; dev-only (default: $TURN_CREDENTIAL)")

	flag.Parse()
	// The ICE overrides are development-only: they are rendered into every room
	// page, so a static TURN credential would be harvestable by anyone who can
	// load any room URL. Outside --dev they are ignored (with a warning) and the
	// built-in STUN servers are served.
	if err := routes.ConfigureICE(*devMode, *iceServers, *turnURL, *turnUsername, *turnCredential); err != nil {
		log.Fatalf("invalid ICE configuration: %v", err)
	}
	routes.StartRouter(*devMode, *port, *basePath)
}
