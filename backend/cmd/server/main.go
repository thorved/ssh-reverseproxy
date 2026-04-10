package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/thorved/ssh-reverseproxy/backend/internal/auth"
	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/database"
	"github.com/thorved/ssh-reverseproxy/backend/internal/proxy"
	"github.com/thorved/ssh-reverseproxy/backend/internal/routes"
)

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	cfg := config.MustLoad()

	db, err := database.Init(cfg)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	authService, err := auth.NewService(cfg, db)
	if err != nil {
		log.Fatalf("init auth service: %v", err)
	}

	proxyServer, err := proxy.NewServer(cfg, db)
	if err != nil {
		log.Fatalf("init ssh proxy: %v", err)
	}

	go func() {
		if err := proxyServer.Run(); err != nil {
			log.Fatalf("ssh proxy stopped: %v", err)
		}
	}()

	router := routes.NewRouter(cfg, db, authService)
	httpServer := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("http api listening on %s", cfg.HTTPListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http api stopped: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	if err := proxyServer.Shutdown(); err != nil {
		log.Printf("ssh shutdown error: %v", err)
	}
}
