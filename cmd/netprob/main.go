package main

import (
	"flag"
	"fmt"
	"os"

	"netprob/internal/buildinfo"
	"netprob/internal/config"

	_ "github.com/joho/godotenv"
)

func main() {
	var (
		configPath  string
		subcommand  string
		showVersion bool
	)

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.StringVar(&subcommand, "mode", "server", "run mode: server or agent")
	flag.BoolVar(&showVersion, "version", false, "print version and build information")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: netprob -mode <server|agent> [flags]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if showVersion {
		fmt.Println(buildinfo.String())
		return
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	switch subcommand {
	case "server":
		runServer(cfg)
	case "agent":
		runAgent(cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s (use 'server' or 'agent')\n", subcommand)
		os.Exit(1)
	}
}
