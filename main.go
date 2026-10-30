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
	"strings"
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

type options struct {
	env      string
	confDir  string
	confFile string
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			logger.Close()
			return
		}
		logger.App().Error(context.Background(), "parse flags failed", zap.Error(err))
		logger.Close()
		os.Exit(2)
	}

	err = run(opts)
	if err != nil {
		logger.App().Error(context.Background(), "server exited with error", zap.Error(err))
	}
	logger.Close()
	if err != nil {
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	fs := flag.NewFlagSet("ginctl", flag.ContinueOnError)
	var opts options
	fs.StringVar(&opts.env, "env", envOrDefault("ENV", config.DefaultEnv), "runtime environment: dev|test|prod (also via ENV)")
	fs.StringVar(&opts.confDir, "c", "etc", "directory holding config_<env>.yaml files")
	fs.StringVar(&opts.confFile, "f", "", "explicit config file; when set it overrides -env/-c")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.env = strings.ToLower(strings.TrimSpace(opts.env))
	if opts.env == "" {
		opts.env = config.DefaultEnv
	}
	return opts, nil
}

func run(opts options) (runErr error) {
	if err := config.ValidateEnv(opts.env); err != nil {
		return err
	}

	configFile := opts.confFile
	if configFile == "" {
		configFile = config.FileForEnv(opts.confDir, opts.env)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("load config %q: %w", configFile, err)
	}

	if err := cfg.Validate(opts.env); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	bg := context.Background()
	logger.App().Info(bg, "config loaded", zap.String("env", opts.env), zap.String("file", configFile))

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

	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
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
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
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
		stop()
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

	select {
	case <-serveDone:
	case <-shutdownCtx.Done():
		logger.App().Warn(bg, "server did not stop before shutdown deadline")
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
