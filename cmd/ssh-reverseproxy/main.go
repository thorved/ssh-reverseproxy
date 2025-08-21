package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"

	"ssh-reverseproxy/internal/config"
	"ssh-reverseproxy/internal/mapping"
	"ssh-reverseproxy/internal/sshproxy"
)

func main() {
	_ = godotenv.Load()

	cfg := config.MustLoad()
	if cfg.DBDSN == "" {
		log.Fatal("SSH_DB_DSN is required (DB-only mode)")
	}

	m, err := mapping.LoadFromDB(context.Background(), cfg.DBDSN, cfg.DBTable)
	if err != nil {
		log.Fatalf("failed to load mapping from DB: %v", err)
	}

	if err := sshproxy.Run(cfg, m); err != nil {
		log.Fatal(err)
	}
}
