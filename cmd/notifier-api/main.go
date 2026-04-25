package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/alexandersustavov/notifier/notifier-api/internal/config"
	"github.com/alexandersustavov/notifier/notifier-api/internal/database"
	httpapp "github.com/alexandersustavov/notifier/notifier-api/internal/http"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := database.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: httpapp.NewRouter(cfg, store),
	}

	log.Printf("notifier-api listening on %s (%s)", cfg.Addr, cfg.Env)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
