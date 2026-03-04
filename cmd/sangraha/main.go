// Command sangraha is a single-binary, S3-compatible object storage system.
package main

import (
	"fmt"
	"os"
)

// Injected at build time via -ldflags.
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Printf("sangraha %s (built %s)\n", version, buildTime)
		return
	}
	fmt.Printf("sangraha %s — S3-compatible object storage\n", version)
	fmt.Println("Run `sangraha --help` for usage.")
}
