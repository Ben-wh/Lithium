package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/lithium/lithium/internal/mail"
)

// fromName is the display name on outgoing email: the company name, or Lithium.
func (s *Server) fromName(ctx context.Context) string {
	st, err := s.store.Settings(ctx)
	if err != nil {
		return "Lithium"
	}
	return brandName(st)
}

// handleSendTestEmail sends a test email to the signed-in admin straight
// away (not through the outbox), so a mistake in the SMTP settings shows up
// on screen rather than in the logs.
func (s *Server) handleSendTestEmail(w http.ResponseWriter, r *http.Request) {
	if s.mailer == nil {
		writeError(w, http.StatusConflict, "email isn't set up on this server")
		return
	}
	user := userFrom(r.Context())
	brand := s.fromName(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	err := s.mailer.Send(ctx, mail.Message{
		FromName: brand,
		From:     s.cfg.Mail.From,
		To:       user.Email,
		Subject:  "Test email from " + brand,
		Body: fmt.Sprintf("This is a test email from %s, sent by %s.\n\n"+
			"If you're reading this, email notifications are set up correctly.\n", brand, user.Name),
	})
	if err != nil {
		s.log.Warn("test email failed", "err", err, "by", user.ID)
		writeError(w, http.StatusBadGateway, "couldn't send: "+err.Error())
		return
	}
	s.log.Info("test email sent", "by", user.ID)
	writeJSON(w, http.StatusOK, map[string]string{"sentTo": user.Email})
}
