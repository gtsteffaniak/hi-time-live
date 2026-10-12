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
	iceServers := flag.String("ice-servers", os.Getenv("ICE_SERVERS"), "JSON array of RTCIceServer objects; overrides the STUN/TURN flags (default: $ICE_SERVERS)")
	turnURL := flag.String("turn-url", os.Getenv("TURN_URL"), "TURN server URL(s), comma separated, e.g. turn:turn.example.com:3478 (default: $TURN_URL)")
	turnUsername := flag.String("turn-username", os.Getenv("TURN_USERNAME"), "TURN username (default: $TURN_USERNAME)")
	turnCredential := flag.String("turn-credential", os.Getenv("TURN_CREDENTIAL"), "TURN credential (default: $TURN_CREDENTIAL)")

	flag.Parse()
	if err := routes.ConfigureICE(*iceServers, *turnURL, *turnUsername, *turnCredential); err != nil {
		log.Fatalf("invalid ICE configuration: %v", err)
	}
	routes.StartRouter(*devMode, *port, *basePath)
}
