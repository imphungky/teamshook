package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type serverConfig struct {
	URL  string
	Port string
}

func NewServer() http.Handler {
	mux := http.NewServeMux()
	AddRoutes(mux)
	var handler http.Handler = mux
	return handler
}

func getServerAddress() serverConfig {
	url, urlExists := os.LookupEnv("URL")
	port, portExists := os.LookupEnv("PORT")
	if !urlExists || !portExists {
		url = "localhost"
		port = "8080"
	}
	return serverConfig{
		URL:  url,
		Port: port,
	}
}

func run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := NewServer()
	serverConfig := getServerAddress()
	httpServer := &http.Server{
		Addr:    net.JoinHostPort(serverConfig.URL, serverConfig.Port),
		Handler: srv,
	}

	g, ctx := errgroup.WithContext(ctx)
	fmt.Printf("Starting server on %s...\n", httpServer.Addr)
	g.Go(func() error {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("error listening and serving: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		fmt.Printf("Shutting down server...\n")
		shutdownCtx := context.Background()
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("error shutting down http server: %w\n", err)
		}
		return nil
	})
	return g.Wait()
}

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error running server: %s\n", err)
		os.Exit(1)
	}
}
