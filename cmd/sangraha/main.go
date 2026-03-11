// Command sangraha is a single-binary, S3-compatible object storage system.
package main

import (
	"github.com/madhavkobal/sangraha/cli"
)

// Injected at build time via -ldflags.
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cli.Execute(version, buildTime)
}
