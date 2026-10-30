// Command server is the entrypoint: load config -> init logger -> build service
// context -> build router -> serve with graceful shutdown.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gin-scaffold/common/logger"
	"gin-scaffold/internal/config"
	"gin-scaffold/internal/router"
	"gin-scaffold/internal/svc"

	"go.uber.org/zap"
)

const (
	defaultConfigFile   = "etc/config_dev.yaml"
	httpShutdownTimeout = 10 * time.Second
)

func main() {
	configFile, err := parseOptions(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			logger.Close()
			return
		}
		logger.App().Error(context.Background(), "parse flags failed", zap.Error(err))
		logger.Close()
		os.Exit(2)
	}

	err = run(configFile)
	if err != nil {
		logger.App().Error(context.Background(), "server exited with error", zap.Error(err))
	}
	logger.Close()
	if err != nil {
		os.Exit(1)
	}
}

func parseOptions(args []string) (string, error) {
	fs := flag.NewFlagSet("ginctl", flag.ContinueOnError)
	configFile := fs.String("f", defaultConfigFile, "config file path")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return *configFile, nil
}

func run(configFile string) (runErr error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("load config %q: %w", configFile, err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	bg := context.Background()
	logger.App().Info(bg, "config loaded", zap.String("env", cfg.Server.Env), zap.String("file", configFile))

	svcCtx, err := svc.NewServiceContext(&cfg)
	if err != nil {
		return fmt.Errorf("build service context: %w", err)
	}
	defer func() {
		if closeErr := svcCtx.Close(); closeErr != nil {
			logger.App().Error(context.Background(), "close service context failed", zap.Error(closeErr))
			if runErr == nil {
				runErr = fmt.Errorf("close service context: %w", closeErr)
			}
		}
	}()
	engine := router.New(svcCtx)

	return serve(engine, &cfg)
}

func serve(handler http.Handler, cfg *config.Config) error {
	bg := context.Background()
	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	srv := &http.Server{
		Addr:     addr,
		Handler:  handler,
		ErrorLog: logger.StdLogger(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listenErr := make(chan error, 1)
	go func() { listenErr <- srv.ListenAndServe() }()
	logger.App().Info(bg, "server starting",
		zap.String("addr", addr),
		zap.String("env", cfg.Server.Env),
		zap.String("gin_mode", cfg.GinMode()),
	)

	var serveErr error
	listenReturned := false
	select {
	case serveErr = <-listenErr:
		listenReturned = true
	case <-ctx.Done():
		stop()
		logger.App().Info(bg, "server shutting down gracefully")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	shutdownErr := srv.Shutdown(shutdownCtx)
	if !listenReturned {
		serveErr = <-listenErr
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	if shutdownErr != nil {
		logger.App().Error(bg, "graceful shutdown failed", zap.Error(shutdownErr))
	}
	if serveErr != nil {
		logger.App().Error(bg, "server error", zap.Error(serveErr))
	}
	logger.App().Info(bg, "server stopped")

	return errors.Join(serveErr, shutdownErr)
}
