package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/mateidumitrascu/typepractice/internal/api"
	"github.com/mateidumitrascu/typepractice/internal/store"
	"github.com/mateidumitrascu/typepractice/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := envOr("DB_PATH", "typepractice.db")
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if len(os.Args) > 1 {
		return runCommand(st, os.Args[1:])
	}

	addr := envOr("ADDR", ":8080")
	basePath := os.Getenv("BASE_PATH")
	handler := api.New(st, basePath, web.Dist())
	slog.Info("listening", "addr", addr, "base_path", basePath, "db", dbPath)
	return http.ListenAndServe(addr, handler)
}

func runCommand(st *store.Store, args []string) error {
	if len(args) == 3 && args[0] == "user" && args[1] == "create" {
		return createUser(st, args[2])
	}
	return fmt.Errorf("unknown command %q (usage: server user create <username>)", strings.Join(args, " "))
}

func createUser(st *store.Store, username string) error {
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
	hash, err := bcrypt.GenerateFromPassword(pw, bcrypt.DefaultCost)
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
