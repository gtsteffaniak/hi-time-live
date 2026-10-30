package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/gtsteffaniak/hi-time-live/routes"
)

func main() {
	devMode := flag.Bool("dev", false, "enable dev mode (hot-reloading and debug logging)")
	port := flag.Int("port", 0, "port to run program on")
	basePath := flag.String("base-path", os.Getenv("BASE_PATH"), "URL prefix to serve under, e.g. /hitime (default: $BASE_PATH)")
	iceServers := flag.String("ice-servers", os.Getenv("ICE_SERVERS"), "JSON array of RTCIceServer objects; dev-only override (default: $ICE_SERVERS)")
	turnURL := flag.String("turn-url", os.Getenv("TURN_URL"), "TURN server URL(s), comma separated (default: $TURN_URL)")
	turnUsername := flag.String("turn-username", os.Getenv("TURN_USERNAME"), "TURN username; dev-only with static credential (default: $TURN_USERNAME)")
	turnCredential := flag.String("turn-credential", os.Getenv("TURN_CREDENTIAL"), "TURN credential; dev-only with static username (default: $TURN_CREDENTIAL)")
	turnRESTSecret := flag.String("turn-rest-secret", os.Getenv("TURN_REST_SECRET"), "shared secret for coturn-style time-limited TURN credentials (default: $TURN_REST_SECRET)")
	turnRESTTTL := flag.Duration("turn-rest-ttl", parseDurationEnv("TURN_REST_TTL", 24*time.Hour), "lifetime of REST-minted TURN credentials (default: 24h)")
	turnRESTUser := flag.String("turn-rest-user", os.Getenv("TURN_REST_USER"), "optional suffix in REST TURN usernames (default: $TURN_REST_USER)")

	flag.Parse()
	// Static ICE/TURN overrides are development-only: they are rendered into every
	// room page, so a static TURN credential would be harvestable by anyone who can
	// load any room URL. Outside --dev they are ignored (with a warning) unless
	// TURN_REST_SECRET mints short-lived per-page credentials.
	if err := routes.ConfigureICE(*devMode, *iceServers, *turnURL, *turnUsername, *turnCredential, *turnRESTSecret, *turnRESTTTL, *turnRESTUser); err != nil {
		log.Fatalf("invalid ICE configuration: %v", err)
	}
	routes.StartRouter(*devMode, *port, *basePath)
}

func parseDurationEnv(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("ignoring invalid %s=%q, using %s", name, raw, fallback)
		return fallback
	}
	return d
}
