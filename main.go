// Command server is the entrypoint: load config -> init logger -> build service
// context -> build router -> serve with graceful shutdown.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gin-scaffold/internal/config"
	"gin-scaffold/internal/router"
	"gin-scaffold/internal/svc"
	"gin-scaffold/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		logger.Error("server exited with error", zap.Error(err))
		os.Exit(1)
	}
}

func run() error {
	var (
		env      = flag.String("env", envOrDefault("ENV", config.DefaultEnv), "runtime environment: dev|test|prod (also via ENV)")
		confDir  = flag.String("c", "etc", "directory holding config_<env>.yaml files")
		confFile = flag.String("f", "", "explicit config file; when set it overrides -env/-c")
	)
	flag.Parse()

	configFile := *confFile
	if configFile == "" {
		configFile = config.FileForEnv(*confDir, *env)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()
	logger.Info("config loaded", zap.String("env", *env), zap.String("file", configFile))

	svcCtx := svc.NewServiceContext(&cfg)
	engine := router.New(svcCtx)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ErrorLog:          logger.StdLogger(),
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", zap.String("addr", addr), zap.String("mode", cfg.Server.Mode))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var serveErr error
	select {
	case serveErr = <-errCh:
		logger.Error("server error", zap.Error(serveErr))
	case <-ctx.Done():
		logger.Info("server shutting down gracefully")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
	logger.Info("server stopped")

	return serveErr
}

// envOrDefault returns the value of env var key, or def when it is unset/empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
