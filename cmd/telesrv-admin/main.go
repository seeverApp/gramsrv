package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	srv, err := newServer(cfg, newReadStore(pool))
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	log.Printf("telesrv-admin listening on %s", cfg.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

type uiConfig struct {
	Addr          string
	PostgresDSN   string
	AdminAPIURL   string
	AdminAPIToken string
	Password      string
	Token         string
	SessionKey    []byte
}

func loadConfig() (uiConfig, error) {
	cfg := uiConfig{
		Addr:          envOr("TELESRV_ADMIN_UI_ADDR", "127.0.0.1:2400"),
		PostgresDSN:   envOr("TELESRV_POSTGRES_DSN", "postgres://telesrv:telesrv@127.0.0.1:5432/telesrv?sslmode=disable"),
		AdminAPIURL:   adminAPIURL(envOr("TELESRV_ADMIN_API_ADDR", "127.0.0.1:2399")),
		AdminAPIToken: os.Getenv("TELESRV_ADMIN_API_TOKEN"),
		Password:      os.Getenv("TELESRV_ADMIN_UI_PASSWORD"),
		Token:         os.Getenv("TELESRV_ADMIN_UI_TOKEN"),
	}
	if cfg.Password == "" && cfg.Token == "" {
		return cfg, fmt.Errorf("TELESRV_ADMIN_UI_PASSWORD or TELESRV_ADMIN_UI_TOKEN is required")
	}
	if strings.TrimSpace(cfg.AdminAPIToken) == "" {
		return cfg, fmt.Errorf("TELESRV_ADMIN_API_TOKEN is required for admin write actions")
	}
	rawKey := os.Getenv("TELESRV_ADMIN_SESSION_KEY")
	if rawKey == "" {
		return cfg, fmt.Errorf("TELESRV_ADMIN_SESSION_KEY is required")
	}
	sum := sha256.Sum256([]byte(rawKey))
	cfg.SessionKey = sum[:]
	return cfg, nil
}

func adminAPIURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = "127.0.0.1:2399"
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/")
	}
	return "http://" + addr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newCommandID(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + hex.EncodeToString(b[:])
}
