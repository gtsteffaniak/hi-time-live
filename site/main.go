package main

import (
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/gtsteffaniak/hi-time-live/routes"
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
	if *devMode {
		opts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
	slog.Debug("Program was run in dev mode which enables debugging and hotloading.")
	routes.StartRouter(*devMode, *logger)
}
