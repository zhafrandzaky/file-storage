// Command server runs the File Storage web service.
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

	"github.com/zhafrandzaky/file-storage/internal/audit"
	"github.com/zhafrandzaky/file-storage/internal/auth"
	"github.com/zhafrandzaky/file-storage/internal/config"
	"github.com/zhafrandzaky/file-storage/internal/database"
	"github.com/zhafrandzaky/file-storage/internal/files"
	"github.com/zhafrandzaky/file-storage/internal/httpapi"
	"github.com/zhafrandzaky/file-storage/internal/storage"
	"github.com/zhafrandzaky/file-storage/internal/users"
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

	db, err := database.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	userStore := users.NewStore(db)
	created, err := userStore.EnsureAdmin(ctx, cfg.Admin.Name, cfg.Admin.Email, cfg.Admin.Password)
	if err != nil {
		return err
	}
	if created {
		slog.Info("admin account created", "email", cfg.Admin.Email)
	}

	objects := storage.NewS3(cfg.S3)
	if err := objects.EnsureBucket(ctx, 20, 2*time.Second); err != nil {
		return err
	}

	sessions := auth.NewSessions(db, cfg.SessionTTL)
	go purgeSessions(ctx, sessions)

	static := webui.FS()
	if static == nil {
		slog.Warn("web UI not built; serving API only")
	}

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.New(httpapi.Deps{
			Objects:        objects,
			Files:          files.NewStore(db),
			Users:          userStore,
			Sessions:       sessions,
			Audit:          audit.New(db),
			Policy:         files.Policy{AllowedExt: cfg.AllowedExt, MaxFileBytes: cfg.MaxFileBytes},
			MaxUploadBytes: cfg.MaxUploadBytes,
			ShareMaxTTL:    cfg.ShareMaxTTL,
			CookieSecure:   cfg.CookieSecure,
			Endpoint:       cfg.S3.Endpoint,
			Static:         static,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "bucket", cfg.S3.Bucket, "endpoint", cfg.S3.Endpoint,
			"share_links", objects.ShareEnabled(), "db", cfg.DBPath)
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

func purgeSessions(ctx context.Context, s *auth.Sessions) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.DeleteExpired(ctx); err != nil {
				slog.Warn("purge sessions", "err", err)
			}
		}
	}
}
