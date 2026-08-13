package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"urockclimbing.com/backend/internal/config"
	"urockclimbing.com/backend/internal/httpserver"
	"urockclimbing.com/backend/internal/platform/database"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/platform/wechat"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := runHealthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	var phoneCipher *securefield.Cipher
	if cfg.PhoneDataKey != "" {
		phoneCipher, err = securefield.New(cfg.PhoneDataKey)
		if err != nil {
			logger.Error("load phone data encryption", "error", err)
			os.Exit(1)
		}
	}

	wechatClient := wechat.NewLoginClient(cfg.WechatAppID, cfg.WechatAppSecret, cfg.WechatLoginMock)
	handler := httpserver.New(httpserver.Dependencies{
		DB:                db,
		Logger:            logger,
		AppEnv:            cfg.AppEnv,
		StartedAt:         time.Now().UTC(),
		PhoneCipher:       phoneCipher,
		AccessTokenSecret: cfg.AccessTokenSecret,
		WechatAppID:       cfg.WechatAppID,
		WechatClient:      wechatClient,
		WechatPhoneClient: wechatClient,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api server started", "address", cfg.HTTPAddr, "environment", cfg.AppEnv)
		serverErr <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api server stopped")
}

func runHealthcheck() error {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("api is not ready")
	}
	return nil
}
