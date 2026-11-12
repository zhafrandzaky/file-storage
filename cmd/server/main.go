// Command server runs the file storage web service.
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

	"github.com/zhafrandzaky/file-storage/internal/config"
	"github.com/zhafrandzaky/file-storage/internal/httpapi"
	"github.com/zhafrandzaky/file-storage/internal/storage"
	"github.com/zhafrandzaky/file-storage/internal/webui"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := storage.NewS3(cfg.S3)
	if err := store.EnsureBucket(ctx, 20, 2*time.Second); err != nil {
		return err
	}

	static := webui.FS()
	if static == nil {
		slog.Warn("web UI not built; serving API only")
	}

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.New(httpapi.Options{
			Store:          store,
			MaxUploadBytes: cfg.MaxUploadBytes,
			Endpoint:       cfg.S3.Endpoint,
			Static:         static,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "bucket", cfg.S3.Bucket, "endpoint", cfg.S3.Endpoint)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}
