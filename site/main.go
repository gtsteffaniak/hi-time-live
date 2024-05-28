package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gtsteffaniak/hi-time-live/routes"
	"github.com/labstack/echo/v4"
)

func main() {
	devMode := flag.Bool("dev", false, "enable dev mode (hot-reloading and debug logging)")
	flag.Parse()
	opts := &slog.HandlerOptions{
		// Use the ReplaceAttr function on the handler options
		// to be able to replace any single attribute in the log output
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// check that we are handling the time key
			if a.Key != slog.TimeKey {
				return a
			}
			t := a.Value.Time()
			// change the value from a time.Time to a String
			// where the string has the correct time format.
			a.Value = slog.StringValue(t.Format(time.DateTime))
			return a
		},
	}
	e := echo.New()
	if *devMode {
		opts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
	slog.Debug("Program was run in dev mode which enables debugging and hotloading.")
	routes.SetupRoutes(e, *devMode, *logger)
	// attempt tls first, fallback to unsecure
	if err := e.StartTLS(":9012", "cert.pem", "key.pem"); err != http.ErrServerClosed {
		slog.Warn("The server is running with HTTP instead of HTTPS, please ensure its accessed behind a reverse proxy with HTTPS.")
		e.Logger.Fatal(e.Start(":9012"))
	}
}
