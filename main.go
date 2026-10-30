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

	"gin-scaffold/infra/logger"
	"gin-scaffold/internal/config"
	"gin-scaffold/internal/router"
	"gin-scaffold/internal/svc"

	"go.uber.org/zap"
)

// Fixed HTTP settings. These are intentionally not exposed as configuration in
// the minimal skeleton; change them here when needed.
const (
	httpReadTimeout       = 15 * time.Second
	httpReadHeaderTimeout = 5 * time.Second
	httpWriteTimeout      = 15 * time.Second
	httpIdleTimeout       = 60 * time.Second
	httpShutdownTimeout   = 10 * time.Second
	httpMaxHeaderBytes    = 1 << 20
)

func main() {
	err := run()
	if err != nil {
		logger.App().Error(context.Background(), "server exited with error", zap.Error(err))
	}
	logger.Close()
	if err != nil {
		os.Exit(1)
	}
}

func run() (runErr error) {
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

	if err := logger.Init(logger.Options{
		RootDir:     cfg.Log.RootDir,
		Level:       cfg.Log.Level,
		Format:      cfg.Log.Format,
		DailyRotate: cfg.Log.DailyRotate,
		Timezone:    cfg.Log.Timezone,
		MaxSizeMB:   cfg.Log.MaxSizeMB,
		MaxAgeDays:  cfg.Log.MaxAgeDays,
		Compress:    cfg.Log.Compress,
	}); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	bg := context.Background()
	logger.App().Info(bg, "config loaded", zap.String("env", *env), zap.String("file", configFile))

	if err := cfg.Validate(*env); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	svcCtx, err := svc.NewServiceContext(&cfg)
	if err != nil {
		return fmt.Errorf("build service context: %w", err)
	}
	defer func() {
		if closeErr := svcCtx.Close(); closeErr != nil {
			logger.App().Error(context.Background(), "close database failed", zap.Error(closeErr))
			if runErr == nil {
				runErr = fmt.Errorf("close database: %w", closeErr)
			}
		}
	}()
	engine := router.New(svcCtx)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ErrorLog:          logger.StdLogger(),
		ReadTimeout:       httpReadTimeout,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    httpMaxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.App().Info(bg, "server starting", zap.String("addr", addr), zap.String("mode", cfg.Server.Mode))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var serveErr error
	select {
	case serveErr = <-errCh:
		logger.App().Error(bg, "server error", zap.Error(serveErr))
	case <-ctx.Done():
		logger.App().Info(bg, "server shutting down gracefully")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.App().Error(bg, "graceful shutdown failed", zap.Error(err))
		if serveErr == nil {
			serveErr = err
		}
	}
	logger.App().Info(bg, "server stopped")

	return serveErr
}

// envOrDefault returns the value of env var key, or def when it is unset/empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
