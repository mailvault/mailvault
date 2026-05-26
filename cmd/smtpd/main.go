package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/mailvault/mailvault/app/smtpd"
)

var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

func main() {
	smtpd.BuildCommit = BuildCommit
	smtpd.BuildTime = BuildTime

	var cfg smtpd.Config
	if err := cfg.Load(""); err != nil {
		panic(fmt.Errorf("loading config: %w", err))
	}

	// OSS defaults: no quota enforcement, no usage metering. Self-hosters get
	// the unmetered server. The commercial overlay supplies builders.
	if err := smtpd.Run(context.Background(), smtpd.Options{Config: cfg}); err != nil {
		slog.Default().Error("smtpd failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
