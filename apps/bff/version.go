package main

// These variables are set at build time via ldflags:
//
//	go build -ldflags="-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)
