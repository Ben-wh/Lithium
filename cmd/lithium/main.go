package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/db"
	"github.com/lithium/lithium/internal/mail"
	"github.com/lithium/lithium/internal/seed"
	"github.com/lithium/lithium/internal/server"
	"github.com/lithium/lithium/internal/store"
	"github.com/lithium/lithium/web"
)

func main() {
	cfg := config.Load()

	// `lithium healthcheck` is used by Docker, since the distroless image has no curl.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck(cfg.Addr))
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// `lithium seed-example` adds the example knowledge base article to an
	// existing install (new installs get it automatically). Handy for dev.
	if len(os.Args) > 1 && os.Args[1] == "seed-example" {
		if err := seedExample(cfg); err != nil {
			log.Error("seed example", "err", err)
			os.Exit(1)
		}
		return
	}

	// `lithium reset-2fa <email>` turns off someone's two-factor from the
	// server, for when the only admin has lost their phone and recovery codes.
	if len(os.Args) > 1 && os.Args[1] == "reset-2fa" {
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: lithium reset-2fa <email>")
			os.Exit(2)
		}
		if err := resetTwoFactor(cfg, os.Args[2]); err != nil {
			log.Error("reset two-factor", "err", err)
			os.Exit(1)
		}
		return
	}

	// `lithium end-lockdown` ends lockdown mode from the server, for when the
	// admin who started it can't sign in.
	if len(os.Args) > 1 && os.Args[1] == "end-lockdown" {
		if err := endLockdown(cfg); err != nil {
			log.Error("end lockdown", "err", err)
			os.Exit(1)
		}
		return
	}

	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	st := store.New(conn)

	if err := bootstrapAdmin(ctx, cfg, st, log); err != nil {
		return err
	}
	go cleanupSessions(ctx, st, log)
	srv, err := server.New(cfg, st, log)
	if err != nil {
		return err
	}
	startEmail(ctx, cfg, st, srv, log)
	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(web.FS()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // room for image uploads
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "db", cfg.DBPath)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
	return nil
}

// bootstrapAdmin creates the first admin account from env vars when the
// database has no users yet (a new install), along with example content.
func bootstrapAdmin(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		log.Warn("no users exist; set LITHIUM_ADMIN_EMAIL and LITHIUM_ADMIN_PASSWORD to create the first admin")
		return nil
	}
	if len(cfg.AdminPassword) < 8 {
		return errors.New("LITHIUM_ADMIN_PASSWORD must be at least 8 characters")
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	u, err := st.CreateUser(ctx, strings.TrimSpace(cfg.AdminEmail), cfg.AdminName, hash, "admin", true)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	log.Info("created admin user", "email", u.Email)
	if err := st.AddAudit(ctx, store.AuditEntry{
		Action: "system.install", Category: "system", Summary: "Set up Lithium and created the first admin",
		ActorName: "System", Target: u.Name + " (" + u.Email + ")", Outcome: store.AuditSuccess,
	}); err != nil {
		log.Warn("write activity log", "err", err)
	}
	// Only this first admin, and only once, may skip mandatory two-factor
	// setup (for trying Lithium out); see SkipFirstAdminTwoFactor.
	if err := st.AllowFirstAdminTwoFactorSkip(ctx, u.ID); err != nil {
		log.Warn("allow first admin to skip two-factor", "err", err)
	}

	// A brand-new install also gets an example knowledge base article.
	if added, err := seed.KnowledgeBase(ctx, st, u.ID); err != nil {
		log.Warn("seed knowledge base", "err", err)
	} else if added {
		log.Info("added example knowledge base article")
	}
	return nil
}

// startEmail runs the outbox in the background. It always runs: email can
// come from LITHIUM_SMTP_* or be set up later by an admin in settings, and
// the outbox asks the server for the current setup before each batch.
func startEmail(ctx context.Context, cfg config.Config, st *store.Store, srv *server.Server, log *slog.Logger) {
	if cfg.Mail.Enabled() {
		if cfg.BaseURL == "" {
			log.Warn("LITHIUM_BASE_URL isn't set, so emails can't link to tickets")
		}
		log.Info("email notifications on", "smtp", fmt.Sprintf("%s:%d", cfg.Mail.Host, cfg.Mail.Port), "tls", cfg.Mail.TLS, "from", cfg.Mail.From)
	} else {
		log.Info("email isn't set by LITHIUM_SMTP_*; admins can set it up in Settings → Email")
	}
	outbox := &mail.Outbox{
		Store: st,
		Setup: srv.OutboxSetup,
		FromName: func(ctx context.Context) string {
			if s, err := st.Settings(ctx); err == nil && s.CompanyName != "" {
				return s.CompanyName
			}
			return "Lithium"
		},
		Log: log,
	}
	go outbox.Run(ctx)
}

func cleanupSessions(ctx context.Context, st *store.Store, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if n, err := st.DeleteExpiredSessions(ctx); err != nil && ctx.Err() == nil {
			log.Error("cleanup sessions", "err", err)
		} else if n > 0 {
			log.Info("removed expired sessions", "count", n)
		}
		// The activity log keeps a year of history.
		if n, err := st.PruneAudit(ctx, time.Now().AddDate(-1, 0, 0)); err != nil && ctx.Err() == nil {
			log.Error("prune activity log", "err", err)
		} else if n > 0 {
			log.Info("pruned activity log", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func healthcheck(addr string) int {
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// seedExample adds the example knowledge base article, credited to the first
// admin. Like on a new install, it only does so if the knowledge base is empty.
func seedExample(cfg config.Config) error {
	ctx := context.Background()
	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	st := store.New(conn)

	users, err := st.ListUsers(ctx, "")
	if err != nil {
		return err
	}
	var authorID int64
	for _, u := range users {
		if u.Role == "admin" {
			authorID = u.ID
			break
		}
	}
	added, err := seed.KnowledgeBase(ctx, st, authorID)
	if err != nil {
		return err
	}
	if added {
		fmt.Println("Added the example article to the knowledge base.")
	} else {
		fmt.Println("The knowledge base already has content, so nothing was added.")
	}
	return nil
}

func resetTwoFactor(cfg config.Config, email string) error {
	ctx := context.Background()
	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	st := store.New(conn)
	u, err := st.UserByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		return fmt.Errorf("find %s: %w", email, err)
	}
	if err := st.ResetTwoFactor(ctx, u.ID, ""); err != nil {
		return err
	}
	fmt.Printf("Two-factor reset for %s. They've been signed out and can set it up again when they next sign in.\n", u.Email)
	return nil
}

func endLockdown(cfg config.Config) error {
	ctx := context.Background()
	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	st := store.New(conn)
	was, err := st.EndLockdown(ctx)
	if err != nil {
		return err
	}
	if !was {
		fmt.Println("Lithium isn't in lockdown mode.")
		return nil
	}
	if err := st.AddAudit(ctx, store.AuditEntry{
		Action: "security.lockdown_end", Category: "settings", Summary: "Ended lockdown mode from the server command line",
		ActorName: "System", Outcome: store.AuditSuccess,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "warning: couldn't write the activity log:", err)
	}
	fmt.Println("Lockdown ended. Everyone can sign in again.")
	return nil
}
