package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mateidumitrascu/typing-practice/internal/tui"
)

// version is set at build time:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/tui
var version = "dev"

func main() {
	server := flag.String("server", "", "server URL (overrides "+tui.ServerEnvVar+" and saved config)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "typepractice — terminal typing practice\n\nusage: %s [flags]\n\nflags:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nenvironment:\n  %s\tserver URL\n", tui.ServerEnvVar)
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("typepractice", version)
		return
	}

	if *server != "" && !tui.ValidServer(tui.NormalizeServer(*server)) {
		fmt.Fprintf(os.Stderr, "error: invalid server URL %q\n", *server)
		os.Exit(1)
	}

	app := tui.NewApp(tui.LoadConfig(), *server)
	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
