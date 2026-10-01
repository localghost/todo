// Command todo runs the todo web app.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
	"todo/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "todo:", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:8811", "address to listen on")
	dbPath := flag.String("db", "todo.db", "path to the SQLite database file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	handler, err := web.New(todo.NewService(store), logger)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "url", "http://"+*addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
