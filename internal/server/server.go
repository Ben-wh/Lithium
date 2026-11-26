// Package server wires up HTTP routes.
package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/backup"
	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/mail"
	"github.com/lithium/lithium/internal/store"
)

type Server struct {
	cfg          config.Config
	store        *store.Store
	log          *slog.Logger
	limiter      *ipLimiter       // login attempts
	resetLimiter *ipLimiter       // password reset requests
	mfaLimiter   *ipLimiter       // two-factor codes
	throttle     *accountThrottle // wrong passwords and codes, per account
	dummyHash    string           // compared against when a login email doesn't exist, to keep timing uniform
	mailer       mail.Sender      // tests only: replaces the SMTP sender (see senderFor)
	updates      *updateChecker
	backups      *backup.Manager
}

func New(cfg config.Config, st *store.Store, log *slog.Logger) (*Server, error) {
	dummy, err := auth.HashPassword("lithium-timing-equaliser")
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:          cfg,
		store:        st,
		log:          log,
		limiter:      newIPLimiter(),
		resetLimiter: newIPLimiter(),
		mfaLimiter:   newIPLimiter(),
		throttle:     newAccountThrottle(),
		dummyHash:    dummy,
		updates:      newUpdateChecker(),
		backups:      backup.New(cfg.DataDir, st, log),
	}, nil
}

// Handler returns the root HTTP handler. frontend is the built SPA; it may be
// empty during backend-only development.
func (s *Server) Handler(frontend fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.Handle("GET /api/auth/me", s.requirePendingSession(http.HandlerFunc(s.handleMe)))
	mux.Handle("POST /api/auth/password", s.requireSession(http.HandlerFunc(s.handleChangePassword)))
	mux.Handle("POST /api/auth/2fa/verify", s.requirePendingSession(http.HandlerFunc(s.handleVerifyTwoFactor)))
	mux.Handle("GET /api/auth/2fa", s.requireSession(http.HandlerFunc(s.handleTwoFactorStatus)))
	mux.Handle("POST /api/auth/2fa/setup", s.requireSession(http.HandlerFunc(s.handleStartTwoFactor)))
	mux.Handle("POST /api/auth/2fa/enable", s.requireSession(http.HandlerFunc(s.handleEnableTwoFactor)))
	mux.Handle("POST /api/auth/2fa/skip", s.requireSession(http.HandlerFunc(s.handleSkipTwoFactor)))
	mux.Handle("POST /api/auth/2fa/recovery-codes", s.requireUser(http.HandlerFunc(s.handleNewRecoveryCodes)))
	mux.Handle("POST /api/auth/2fa/disable", s.requireUser(http.HandlerFunc(s.handleDisableTwoFactor)))
	mux.HandleFunc("POST /api/auth/password-reset-request", s.handleRequestPasswordReset)
	mux.HandleFunc("GET /api/public/banner", s.handlePublicBanner) // no sign-in needed
	mux.HandleFunc("GET /api/public/branding", s.handlePublicBranding)
	mux.HandleFunc("GET /api/public/branding/logo", s.handleGetLogo)

	mux.Handle("POST /api/tickets", s.requireUser(http.HandlerFunc(s.handleCreateTicket)))
	mux.Handle("GET /api/tickets", s.requireUser(http.HandlerFunc(s.handleListTickets)))
	mux.Handle("GET /api/tickets/stats", s.requireUser(http.HandlerFunc(s.handleTicketStats)))
	mux.Handle("GET /api/tickets/{id}", s.requireUser(http.HandlerFunc(s.handleGetTicket)))
	mux.Handle("PATCH /api/tickets/{id}", s.requireStaff(http.HandlerFunc(s.handleUpdateTicket)))
	mux.Handle("POST /api/tickets/bulk", s.requireStaff(http.HandlerFunc(s.handleBulkUpdateTickets)))
	mux.Handle("POST /api/tickets/{id}/reopen", s.requireUser(http.HandlerFunc(s.handleReopenTicket)))
	mux.Handle("GET /api/tickets/{id}/links", s.requireStaff(http.HandlerFunc(s.handleTicketLinks)))
	mux.Handle("GET /api/tickets/{id}/assignable", s.requireStaff(http.HandlerFunc(s.handleAssignable)))
	mux.Handle("PUT /api/tickets/{id}/parent", s.requireStaff(http.HandlerFunc(s.handleSetParent)))
	mux.Handle("GET /api/tickets/{id}/tree", s.requireStaff(http.HandlerFunc(s.handleIncidentTree)))
	mux.Handle("GET /api/tickets/{id}/comments", s.requireUser(http.HandlerFunc(s.handleListComments)))
	mux.Handle("POST /api/tickets/{id}/comments", s.requireUser(http.HandlerFunc(s.handleAddComment)))
	mux.Handle("GET /api/tickets/{id}/mentionable", s.requireUser(http.HandlerFunc(s.handleMentionable)))
	mux.Handle("GET /api/tickets/{id}/mentionable-groups", s.requireStaff(http.HandlerFunc(s.handleMentionableGroups)))
	mux.Handle("GET /api/notifications", s.requireUser(http.HandlerFunc(s.handleListNotifications)))
	mux.Handle("GET /api/notifications/unread", s.requireUser(http.HandlerFunc(s.handleUnreadCount)))
	mux.Handle("POST /api/notifications/read", s.requireUser(http.HandlerFunc(s.handleMarkRead)))
	mux.Handle("GET /api/attachments/{id}", s.requireUser(http.HandlerFunc(s.handleGetAttachment)))
	mux.Handle("PUT /api/profile/avatar", s.requireUser(http.HandlerFunc(s.handleSetPresetAvatar)))
	mux.Handle("POST /api/profile/avatar", s.requireUser(http.HandlerFunc(s.handleUploadAvatar)))
	mux.Handle("PUT /api/profile/email-notifications", s.requireUser(http.HandlerFunc(s.handleSetEmailNotifications)))
	mux.Handle("GET /api/avatars/{id}", s.requireUser(http.HandlerFunc(s.handleGetAvatar)))
	mux.Handle("GET /api/users", s.requireStaff(http.HandlerFunc(s.handleUserDirectory)))
	mux.Handle("GET /api/reports", s.requireStaff(http.HandlerFunc(s.handleReport)))
	mux.Handle("GET /api/reports/export", s.requireStaff(http.HandlerFunc(s.handleExport)))

	// Knowledge base: everyone reads (users see published, public articles); staff write.
	user := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireUser(h)) }
	staff := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireStaff(h)) }
	user("GET /api/kb/collections", s.handleListKBCollections)
	staff("POST /api/kb/collections", s.handleCreateKBCollection)
	staff("PUT /api/kb/collections/{id}", s.handleUpdateKBCollection)
	staff("DELETE /api/kb/collections/{id}", s.handleDeleteKBCollection)
	user("GET /api/kb/articles", s.handleListKBArticles)
	staff("POST /api/kb/articles", s.handleCreateKBArticle)
	user("GET /api/kb/articles/{id}", s.handleGetKBArticle)
	staff("PUT /api/kb/articles/{id}", s.handleUpdateKBArticle)
	staff("DELETE /api/kb/articles/{id}", s.handleDeleteKBArticle)
	staff("POST /api/kb/articles/{id}/images", s.handleUploadKBImage)
	user("GET /api/kb/images/{name}", s.handleGetKBImage)

	mux.Handle("GET /api/people", s.requireStaff(http.HandlerFunc(s.handleListPeople)))
	mux.Handle("GET /api/people/{id}", s.requireStaff(http.HandlerFunc(s.handlePerson)))
	mux.Handle("GET /api/categories", s.requireUser(http.HandlerFunc(s.handleListCategories)))
	mux.Handle("GET /api/settings", s.requireUser(http.HandlerFunc(s.handleGetSettings)))

	admin := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireAdmin(h)) }
	admin("GET /api/admin/users", s.handleListUsers)
	admin("POST /api/admin/users", s.handleCreateUser)
	admin("POST /api/admin/users/{id}/password", s.handleSetPassword)
	admin("PATCH /api/admin/users/{id}", s.handleUpdateUser)
	admin("DELETE /api/admin/users/{id}", s.handleDeleteUser)
	admin("POST /api/admin/users/{id}/2fa/reset", s.handleResetTwoFactor)
	admin("GET /api/admin/password-resets", s.handleListResetRequests)
	admin("POST /api/admin/password-resets/{id}/dismiss", s.handleDismissResetRequest)
	admin("GET /api/admin/categories", s.handleAdminListCategories)
	admin("POST /api/admin/categories", s.handleCreateCategory)
	admin("PATCH /api/admin/categories/{id}", s.handleUpdateCategory)
	admin("DELETE /api/admin/categories/{id}", s.handleDeleteCategory)
	admin("PATCH /api/admin/settings", s.handleUpdateSettings)
	admin("POST /api/admin/branding/logo", s.handleUploadLogo)
	admin("DELETE /api/admin/branding/logo", s.handleDeleteLogo)
	admin("POST /api/admin/email/test", s.handleSendTestEmail)
	admin("GET /api/admin/email", s.handleGetEmailSettings)
	admin("PUT /api/admin/email", s.handleSaveEmailSettings)
	admin("DELETE /api/admin/email", s.handleDeleteEmailSettings)
	admin("GET /api/admin/activity", s.handleActivity)
	admin("GET /api/admin/lockdown", s.handleGetLockdown)
	admin("POST /api/admin/lockdown", s.handleStartLockdown)
	admin("POST /api/admin/lockdown/end", s.handleEndLockdown)
	admin("GET /api/admin/ticket-templates", s.handleListTemplates)
	admin("POST /api/admin/ticket-templates", s.handleSaveTemplate)
	admin("PUT /api/admin/ticket-templates/{id}", s.handleSaveTemplate)
	admin("DELETE /api/admin/ticket-templates/{id}", s.handleDeleteTemplate)
	admin("GET /api/admin/groups", s.handleListGroups)
	admin("POST /api/admin/groups", s.handleCreateGroup)
	admin("PUT /api/admin/groups/{id}", s.handleUpdateGroup)
	admin("DELETE /api/admin/groups/{id}", s.handleDeleteGroup)
	admin("GET /api/admin/updates", s.handleCheckUpdates)
	admin("GET /api/admin/backups", s.handleGetBackups)
	admin("PUT /api/admin/backups/settings", s.handleSaveBackupSettings)
	admin("POST /api/admin/backups", s.handleCreateBackup)
	admin("GET /api/admin/backups/{name}/download", s.handleDownloadBackup)
	admin("DELETE /api/admin/backups/{name}", s.handleDeleteBackup)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	mux.Handle("/", spaHandler(frontend))

	return s.logRequests(securityHeaders(requireJSONForWrites(s.auditRequests(mux))))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
