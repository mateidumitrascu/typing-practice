package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/mateidumitrascu/typing-practice/internal/api"
	"github.com/mateidumitrascu/typing-practice/internal/config"
	"github.com/mateidumitrascu/typing-practice/internal/store"
	"github.com/mateidumitrascu/typing-practice/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogging(cfg)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if len(os.Args) > 1 {
		return runCommand(st, cfg, os.Args[1:])
	}

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      api.New(st, cfg, web.Dist()),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("listening", "addr", cfg.Addr, "base_path", cfg.BasePath, "db", cfg.DBPath)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}

func setupLogging(cfg config.Config) {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var h slog.Handler
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func runCommand(st *store.Store, cfg config.Config, args []string) error {
	if len(args) == 3 && args[0] == "user" && args[1] == "create" {
		return createUser(st, cfg, args[2])
	}
	return fmt.Errorf("unknown command %q (usage: server user create <username>)", strings.Join(args, " "))
}

func createUser(st *store.Store, cfg config.Config, username string) error {
	pw, err := readPassword("Password: ")
	if err != nil {
		return err
	}
	pw2, err := readPassword("Confirm: ")
	if err != nil {
		return err
	}
	if string(pw) != string(pw2) {
		return fmt.Errorf("passwords do not match")
	}
	if len(pw) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword(pw, cfg.BcryptCost)
	if err != nil {
		return err
	}
	if err := st.CreateUser(context.Background(), username, string(hash)); err != nil {
		return err
	}
	fmt.Printf("user %q created\n", username)
	return nil
}

func readPassword(prompt string) ([]byte, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		pw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		return pw, err
	}
	var pw string
	_, err := fmt.Scanln(&pw)
	return []byte(pw), err
}
