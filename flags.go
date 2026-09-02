package main

import (
	"flag"
	"fmt"
	"os"

	s "github.com/FacileStudio/nacelle-tui/internal/settings"
)

// fromFlags is the settings layer the command line supplies, with version
// handling wrapped around it.
func fromFlags() Config {
	showVersion := flag.Bool("version", false, "print the version and exit")
	config := s.FromFlags(s.Defaults(""))
	if *showVersion {
		fmt.Println("nacelle", version)
		os.Exit(0)
	}
	return config
}
