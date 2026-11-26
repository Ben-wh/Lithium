package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lithium/lithium/internal/store"
)

// SLA rules set first-response and resolution targets for tickets by
// category, priority, or both. Staff see how each ticket is doing; admins
// are alerted when a target is close and when it's missed.

const maxSLAMinutes = 365 * 24 * 60

func (s *Server) handleListSLARules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.SLARules(r.Context())
	if err != nil {
		s.log.Error("list sla rules", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

// slaRuleInput reads and checks a rule; a non-empty message is a 400.
func (s *Server) slaRuleInput(w http.ResponseWriter, r *http.Request) (store.SLARuleInput, bool) {
	var req struct {
		CategoryID           *int64 `json:"categoryId"`
		Priority             string `json:"priority"`
		FirstResponseMinutes *int   `json:"firstResponseMinutes"`
		ResolutionMinutes    *int   `json:"resolutionMinutes"`
	}
	if !decodeJSON(w, r, &req) {
		return store.SLARuleInput{}, false
	}
	in := store.SLARuleInput{Priority: req.Priority, FirstResponseMinutes: req.FirstResponseMinutes, ResolutionMinutes: req.ResolutionMinutes}
	if req.CategoryID != nil {
		in.CategoryID = *req.CategoryID
	}
	bad := func(msg string) (store.SLARuleInput, bool) {
		writeError(w, http.StatusBadRequest, msg)
		return in, false
	}
	if in.Priority != "" && !validPriorities[in.Priority] {
		return bad("unknown priority")
	}
	if in.FirstResponseMinutes == nil && in.ResolutionMinutes == nil {
		return bad("set a first response or resolution target")
	}
	for _, m := range []*int{in.FirstResponseMinutes, in.ResolutionMinutes} {
		if m != nil && (*m < 1 || *m > maxSLAMinutes) {
			return bad("targets must be between 1 minute and 365 days")
		}
	}
	if f, res := in.FirstResponseMinutes, in.ResolutionMinutes; f != nil && res != nil && *f > *res {
		return bad("the first response target can't be longer than the resolution target")
	}
	if in.CategoryID != 0 {
		if _, err := s.store.CategoryByID(r.Context(), in.CategoryID); errors.Is(err, store.ErrNotFound) {
			return bad("category not found")
		} else if err != nil {
			s.log.Error("lookup category", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return in, false
		}
	}
	return in, true
}

func (s *Server) handleCreateSLARule(w http.ResponseWriter, r *http.Request) {
	in, ok := s.slaRuleInput(w, r)
	if !ok {
		return
	}
	id, err := s.store.CreateSLARule(r.Context(), in)
	if errors.Is(err, store.ErrDuplicateRule) {
		writeError(w, http.StatusConflict, "there's already a rule for that category and priority")
		return
	} else if err != nil {
		s.log.Error("create sla rule", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.writeSLARule(w, r, id, http.StatusCreated)
}

func (s *Server) handleUpdateSLARule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := s.slaRuleInput(w, r)
	if !ok {
		return
	}
	switch err := s.store.UpdateSLARule(r.Context(), id, in); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "rule not found")
		return
	case errors.Is(err, store.ErrDuplicateRule):
		writeError(w, http.StatusConflict, "there's already a rule for that category and priority")
		return
	case err != nil:
		s.log.Error("update sla rule", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.writeSLARule(w, r, id, http.StatusOK)
}

func (s *Server) writeSLARule(w http.ResponseWriter, r *http.Request, id int64, status int) {
	rules, err := s.store.SLARules(r.Context())
	if err != nil {
		s.log.Error("list sla rules", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	for _, rule := range rules {
		if rule.ID == id {
			writeJSON(w, status, rule)
			return
		}
	}
	writeError(w, http.StatusNotFound, "rule not found")
}

func (s *Server) handleDeleteSLARule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteSLARule(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	} else if err != nil {
		s.log.Error("delete sla rule", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Showing SLAs to staff ---

func categoryIDOf(c *store.CategoryRef) int64 {
	if c == nil {
		return 0
	}
	return c.ID
}

// withSLA fills in a ticket's SLA when the viewer is staff.
func (s *Server) withSLA(ctx context.Context, viewer store.User, t *store.Ticket) {
	if !isStaff(viewer) {
		return
	}
	rules, err := s.store.SLARules(ctx)
	if err != nil {
		s.log.Error("list sla rules", "err", err)
		return
	}
	t.SLA = store.ComputeSLA(rules, categoryIDOf(t.Category), t.Priority, t.Status, t.Clock, time.Now())
}

// withSLAs does the same for a list.
func (s *Server) withSLAs(ctx context.Context, viewer store.User, ts []store.TicketSummary) {
	if !isStaff(viewer) || len(ts) == 0 {
		return
	}
	rules, err := s.store.SLARules(ctx)
	if err != nil {
		s.log.Error("list sla rules", "err", err)
		return
	}
	now := time.Now()
	for i := range ts {
		t := &ts[i]
		t.SLA = store.ComputeSLA(rules, categoryIDOf(t.Category), t.Priority, t.Status, t.Clock, now)
	}
}

// --- Alerts ---

// slaAlertGrace: a threshold crossed longer ago than this is recorded
// without an alert. Adding a rule then doesn't flood admins about every old
// ticket it applies to, and those tickets still show as overdue.
const slaAlertGrace = 30 * time.Minute

// CheckSLAs sends admins any SLA warnings and breaches that are due.
func (s *Server) CheckSLAs(ctx context.Context, now time.Time) {
	rules, err := s.store.SLARules(ctx)
	if err != nil || len(rules) == 0 {
		if err != nil && ctx.Err() == nil {
			s.log.Error("list sla rules", "err", err)
		}
		return
	}
	candidates, err := s.store.SLACandidates(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Error("list sla candidates", "err", err)
		}
		return
	}
	var admins []int64
	for _, c := range candidates {
		r := store.MatchSLARule(rules, c.CategoryID, c.Priority)
		if r == nil {
			continue
		}
		elapsed := c.Clock.Elapsed(now)
		type check struct {
			minutes *int
			warn    int
			breach  int
			what    string
			skip    bool
		}
		checks := []check{
			{r.FirstResponseMinutes, store.AlertFirstResponseWarning, store.AlertFirstResponseBreach, "First response", c.Clock.FirstResponseSeconds != nil},
			{r.ResolutionMinutes, store.AlertResolutionWarning, store.AlertResolutionBreach, "Resolution", false},
		}
		var add int
		var alerts []string
		for _, ch := range checks {
			if ch.minutes == nil || ch.skip {
				continue
			}
			target := time.Duration(*ch.minutes) * time.Minute
			warnAt := time.Duration(float64(target) * store.SLAWarningShare)
			switch {
			case elapsed >= target && c.Clock.Alerts&ch.breach == 0:
				add |= ch.breach | ch.warn // a breach covers the warning too
				if elapsed-target <= slaAlertGrace {
					alerts = append(alerts, fmt.Sprintf("%s is overdue (target %s)", ch.what, formatMinutes(*ch.minutes)))
				}
			case elapsed >= warnAt && elapsed < target && c.Clock.Alerts&ch.warn == 0:
				add |= ch.warn
				if elapsed-warnAt <= slaAlertGrace {
					alerts = append(alerts, fmt.Sprintf("%s is due in %s", ch.what, formatDuration(target-elapsed)))
				}
			}
		}
		if add == 0 {
			continue
		}
		// Record first, so a failed notification never repeats forever.
		if err := s.store.AddSLAAlerts(ctx, c.ID, add); err != nil {
			s.log.Error("record sla alerts", "err", err, "ticket_id", c.ID)
			continue
		}
		if len(alerts) == 0 {
			continue
		}
		if admins == nil {
			if admins, err = s.store.AdminIDs(ctx); err != nil {
				s.log.Error("list admins", "err", err)
				return
			}
		}
		t, err := s.store.TicketByID(ctx, c.ID)
		if err != nil {
			s.log.Error("get ticket", "err", err, "ticket_id", c.ID)
			continue
		}
		rs := newRecipients(0)
		for _, id := range admins {
			rs.add(id)
		}
		for _, a := range alerts {
			s.log.Info("sla alert", "ticket_id", c.ID, "alert", a)
			s.notify(ctx, rs, t, "sla", a)
		}
	}
}

// WatchSLAs checks SLAs once a minute until ctx ends.
func (s *Server) WatchSLAs(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		s.CheckSLAs(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// formatMinutes: 45 → "45m", 240 → "4h", 1500 → "1d 1h".
func formatMinutes(m int) string { return formatDuration(time.Duration(m) * time.Minute) }

func formatDuration(d time.Duration) string {
	m := int(d.Round(time.Minute) / time.Minute)
	if m < 1 {
		m = 1
	}
	days, hours, mins := m/(24*60), m/60%24, m%60
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", mins)
}
