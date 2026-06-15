// Command server is the entrypoint: load config -> build service context ->
// build router -> serve with graceful shutdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gin-scaffold/internal/config"
	"gin-scaffold/internal/router"
	"gin-scaffold/internal/svc"
)

func main() {
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

	cfg := config.MustLoad(configFile)
	slog.Info("config loaded", "env", *env, "file", configFile)

	svcCtx := svc.NewServiceContext(cfg)
	engine := router.New(svcCtx)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", addr, "mode", cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Block until SIGINT/SIGTERM, then drain in-flight requests.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("server shutting down gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
	slog.Info("server stopped")
}

// envOrDefault returns the value of env var key, or def when it is unset/empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
