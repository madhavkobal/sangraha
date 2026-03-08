package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	adminapi "github.com/madhavkobal/sangraha/internal/api/admin"
	"github.com/madhavkobal/sangraha/internal/api/middleware"
	s3api "github.com/madhavkobal/sangraha/internal/api/s3"
	"github.com/madhavkobal/sangraha/internal/audit"
	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/backend/localfs"
	"github.com/madhavkobal/sangraha/internal/config"
	metabbolt "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
	"github.com/madhavkobal/sangraha/internal/storage"
)

var (
	flagConfigFile string
	flagDev        bool

	// binaryVersion and binaryBuildTime are set from Execute() via main.go ldflags.
	binaryVersion   = "dev"
	binaryBuildTime = "unknown"
)

// serverCmd is the parent command for server operations.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the sangraha server",
}

// serverStartCmd starts the server.
var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the sangraha server",
	RunE:  runServerStart,
}

func init() {
	serverStartCmd.Flags().StringVar(&flagConfigFile, "config", "", "path to config file")
	serverStartCmd.Flags().BoolVar(&flagDev, "dev", false, "development mode (text logging, no TLS)")
	serverCmd.AddCommand(serverStartCmd)
}

// serverDeps holds all initialised subsystems.
type serverDeps struct {
	engine   *storage.Engine
	keyStore *auth.KeyStore
	auditor  *audit.Logger
	tlsCfg   *tls.Config
}

func runServerStart(cmd *cobra.Command, _ []string) error {
	cfg, err := loadAndValidateConfig()
	if err != nil {
		return err
	}

	deps, cleanup, err := initDeps(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	return serveUntilSignal(cfg, deps)
}

func loadAndValidateConfig() (*config.Config, error) {
	cfg, err := config.Load(flagConfigFile)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	level, err := zerolog.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	if flagDev || cfg.Logging.Format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(level)
	} else {
		zerolog.SetGlobalLevel(level)
	}
	if err = config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func initDeps(ctx context.Context, cfg *config.Config) (*serverDeps, func(), error) {
	auditor, err := audit.New(cfg.Logging.AuditLog)
	if err != nil {
		return nil, nil, fmt.Errorf("open audit log: %w", err)
	}

	meta, err := metabbolt.Open(cfg.Metadata.Path)
	if err != nil {
		_ = auditor.Close()
		return nil, nil, fmt.Errorf("open metadata store: %w", err)
	}

	be, err := localfs.New(cfg.Storage.DataDir)
	if err != nil {
		_ = meta.Close()
		_ = auditor.Close()
		return nil, nil, fmt.Errorf("open storage backend: %w", err)
	}

	engine := storage.New(be, meta, cfg.Auth.RootAccessKey)
	keyStore := auth.NewKeyStore(meta)

	if rootSecret := cfg.Auth.RootSecretKey; rootSecret != "" {
		if err = keyStore.UpsertKey(ctx, cfg.Auth.RootAccessKey, rootSecret, "root", true); err != nil {
			_ = meta.Close()
			_ = auditor.Close()
			return nil, nil, fmt.Errorf("provision root key: %w", err)
		}
	}

	var tlsCfg *tls.Config
	if cfg.Server.TLS.Enabled && !flagDev {
		tlsCfg, err = middleware.BuildTLSConfig(
			cfg.Server.TLS.CertFile,
			cfg.Server.TLS.KeyFile,
			cfg.Server.TLS.AutoSelfSigned,
		)
		if err != nil {
			_ = meta.Close()
			_ = auditor.Close()
			return nil, nil, fmt.Errorf("build TLS config: %w", err)
		}
	}

	cleanup := func() {
		_ = meta.Close()
		_ = auditor.Close()
	}
	return &serverDeps{engine: engine, keyStore: keyStore, auditor: auditor, tlsCfg: tlsCfg}, cleanup, nil
}

func serveUntilSignal(cfg *config.Config, deps *serverDeps) error {
	rateLimitRPS := cfg.Limits.RateLimitRPS
	if rateLimitRPS <= 0 {
		rateLimitRPS = 1000
	}
	scheme := "http"
	if cfg.Server.TLS.Enabled {
		scheme = "https"
	}
	addr := cfg.Server.S3Address
	if len(addr) > 0 && addr[0] == ':' {
		addr = "localhost" + addr
	}
	serverURL := scheme + "://" + addr

	s3Handler := s3api.New(deps.engine, deps.keyStore, deps.auditor, rateLimitRPS)
	adminHandler := adminapi.New(deps.keyStore, deps.engine, deps.auditor, binaryVersion, binaryBuildTime, serverURL, cfg)

	s3Srv := newHTTPServer(cfg.Server.S3Address, s3Handler)
	adminSrv := newHTTPServer(cfg.Server.AdminAddress, adminHandler)
	if deps.tlsCfg != nil {
		s3Srv.TLSConfig = deps.tlsCfg
	}

	errCh := make(chan error, 2)
	go startServer(s3Srv, deps.tlsCfg != nil, cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile, errCh)
	go startServer(adminSrv, false, "", "", errCh)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("shutting down")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = s3Srv.Shutdown(ctx)
	_ = adminSrv.Shutdown(ctx)
	log.Info().Msg("server stopped")
	return nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

func startServer(srv *http.Server, useTLS bool, certFile, keyFile string, errCh chan<- error) {
	log.Info().Str("address", srv.Addr).Bool("tls", useTLS).Msg("starting server")
	var err error
	if useTLS {
		err = srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		errCh <- fmt.Errorf("server %s: %w", srv.Addr, err)
	}
}
