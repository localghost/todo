// Command todo runs the todo web app.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
	"todo/internal/web"
)

func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "users" {
		err = runUsers(os.Args[2:], os.Stdin, os.Stdout)
	} else {
		err = run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "todo:", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:8811", "address to listen on")
	dbPath := flag.String("db", "todo.db", "path to the SQLite database file")
	trustProxy := flag.Bool("trust-proxy", false, "take the client IP from X-Forwarded-For (only behind an HTTPS proxy that sets it)")
	deleteOldItems := flag.Bool("delete-old-items", false, "allow deleting items from before user accounts when the database is upgraded")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := sqlite.OpenWith(*dbPath, sqlite.Options{DeleteOldItems: *deleteOldItems})
	if err != nil {
		return err
	}
	defer store.Close()

	accounts := auth.NewService(store)
	key, err := store.SigningKey(context.Background())
	if err != nil {
		return err
	}
	handler, err := web.New(todo.NewService(store), accounts, logger, web.WithSigningKey(key), web.WithTrustProxy(*trustProxy))
	if err != nil {
		return err
	}

	// Open the port first, so a port that is in use fails before "listening" is logged.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore default handling: a second Ctrl+C stops at once.
	context.AfterFunc(ctx, stop)

	go cleanSessions(ctx, accounts, logger)

	return serve(ctx, ln, handler, logger)
}

// serve runs the HTTP server on ln until ctx is done, then shuts it down.
func serve(ctx context.Context, ln net.Listener, h http.Handler, logger *slog.Logger) error {
	// Timeouts keep slow or idle clients from holding connections open forever.
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	logger.Info("listening", "url", "http://"+ln.Addr().String())

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// cleanSessions deletes expired sessions now and then every hour.
func cleanSessions(ctx context.Context, accounts *auth.Service, logger *slog.Logger) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if _, err := accounts.CleanUp(ctx); err != nil && ctx.Err() == nil {
			logger.Error("clean up sessions", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
