package server

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lithium/lithium/internal/store"
)

// Mentions are stored in comment text as @[Display Name](userID) for a
// person or @[Group Name](g:groupID) for a group, so names with spaces work
// and the display survives renames.
var mentionRe = regexp.MustCompile(`@\[([^\]\n]{1,100})\]\((g:)?(\d+)\)`)
var groupMentionRe = regexp.MustCompile(`@\[([^\]\n]{1,100})\]\(g:\d+\)`)

// stripMentions turns all mention tokens into plain "@Name" text.
func stripMentions(body string) string {
	return mentionRe.ReplaceAllString(body, "@$1")
}

// stripGroupMentions turns group mention tokens into plain text (only staff
// can mention groups).
func stripGroupMentions(body string) string {
	return groupMentionRe.ReplaceAllString(body, "@$1")
}

// mentionsAllowed reports whether regular (non-staff) users may @mention.
// Staff always can.
func (s *Server) mentionsAllowed(ctx context.Context) (bool, error) {
	st, err := s.store.Settings(ctx)
	return st.AllowUserMentions, err
}

// mentionedIDs returns the people and groups mentioned in a comment.
func mentionedIDs(body string) (users, groups []int64) {
	seen := map[string]bool{}
	for _, m := range mentionRe.FindAllStringSubmatch(body, -1) {
		id, err := strconv.ParseInt(m[3], 10, 64)
		if err != nil || seen[m[2]+m[3]] {
			continue
		}
		seen[m[2]+m[3]] = true
		if m[2] == "g:" {
			groups = append(groups, id)
		} else {
			users = append(users, id)
		}
	}
	return users, groups
}

// canSee reports whether a user can see a ticket (and, for internal notes,
// the note, which only staff can).
func (s *Server) canSee(ctx context.Context, u store.User, t store.Ticket, internal bool) bool {
	if internal && !isStaff(u) {
		return false
	}
	ok, err := s.userCanView(ctx, u, t)
	return err == nil && ok
}

// recipients collects distinct user IDs, skipping zero and the actor.
type recipients struct {
	actor int64
	ids   []int64
	seen  map[int64]bool
}

func newRecipients(actor int64) *recipients {
	return &recipients{actor: actor, seen: map[int64]bool{}}
}

func (r *recipients) add(id int64) {
	if id == 0 || id == r.actor || r.seen[id] {
		return
	}
	r.seen[id] = true
	r.ids = append(r.ids, id)
}

func (s *Server) notify(ctx context.Context, rs *recipients, t store.Ticket, kind, summary string) {
	if len(rs.ids) == 0 {
		return
	}
	var email *store.EmailContent
	if s.emailOn(ctx) {
		e, err := s.notificationEmail(ctx, rs.actor, t, summary)
		if err != nil {
			s.log.Error("compose notification email", "err", err, "ticket_id", t.ID)
		} else {
			email = &e
		}
	}
	if err := s.store.Notify(ctx, rs.ids, t.ID, rs.actor, kind, summary, email); err != nil {
		// Notifications are best-effort: the reply or change itself succeeded.
		s.log.Error("notify", "err", err, "ticket_id", t.ID, "kind", kind)
	}
}

// emailOn reports whether notification emails should be queued: SMTP is
// set up and an admin hasn't turned them off. People can still opt out
// individually (checked when queueing).
func (s *Server) emailOn(ctx context.Context) bool {
	if !s.cfg.Mail.Enabled() {
		return false
	}
	st, err := s.store.Settings(ctx)
	if err != nil {
		s.log.Error("get settings", "err", err)
		return false
	}
	return st.EmailNotifications
}

// notificationEmail is the email version of a notification: who did what,
// on which ticket, and a link to it. It never includes reply or note text,
// and in sensitive mode it leaves out the ticket's subject too.
func (s *Server) notificationEmail(ctx context.Context, actorID int64, t store.Ticket, summary string) (store.EmailContent, error) {
	st, err := s.store.Settings(ctx)
	if err != nil {
		return store.EmailContent{}, err
	}
	brand := brandName(st)
	actor := "Someone"
	if u, err := s.store.UserByID(ctx, actorID); err == nil {
		actor = u.Name
	}
	ref := st.TicketPrefix + "-" + strconv.FormatInt(t.ID, 10)

	e := store.EmailContent{Line: fmt.Sprintf("%s %s.", actor, summary)}
	var b strings.Builder
	if st.SensitiveMode {
		e.Subject = "[" + ref + "] New activity"
		fmt.Fprintf(&b, "%s\n", ref)
	} else {
		e.Subject = "[" + ref + "] " + t.Subject
		fmt.Fprintf(&b, "%s: %s\n", ref, t.Subject)
	}
	if s.cfg.BaseURL != "" {
		fmt.Fprintf(&b, "\nView the ticket: %s/tickets/%d\n", s.cfg.BaseURL, t.ID)
	} else {
		fmt.Fprintf(&b, "\nSign in to %s to view it.\n", brand)
	}
	fmt.Fprintf(&b, "\n-- \nYou're getting this because you're involved in this ticket in %s.\n", brand)
	e.Rest = b.String()
	return e, nil
}

// brandName is what emails call the workspace: the company name, or Lithium.
func brandName(st store.Settings) string {
	if st.CompanyName != "" {
		return st.CompanyName
	}
	return "Lithium"
}

// notifyComment tells people about a new reply or internal note. Anyone
// @mentioned gets "mentioned you" instead of the general notification.
func (s *Server) notifyComment(ctx context.Context, t store.Ticket, author store.User, c store.Comment) {
	where := ""
	if c.Internal {
		where = " in an internal note"
	}
	userIDs, groupIDs := mentionedIDs(c.Body)

	// People mentioned directly.
	mentioned := newRecipients(author.ID)
	for _, id := range userIDs {
		u, err := s.store.UserByID(ctx, id)
		if err == nil && s.canSee(ctx, u, t, c.Internal) {
			mentioned.add(id)
		}
	}
	s.notify(ctx, mentioned, t, "mention", "mentioned you"+where)

	// Members of mentioned groups who can see the ticket, unless already told.
	notified := map[int64]bool{}
	for id := range mentioned.seen {
		notified[id] = true
	}
	for _, gid := range groupIDs {
		g, err := s.store.GroupByID(ctx, gid)
		if err != nil {
			continue
		}
		members := newRecipients(author.ID)
		for _, m := range g.Members {
			if notified[m.ID] {
				continue
			}
			u, err := s.store.UserByID(ctx, m.ID)
			if err == nil && s.canSee(ctx, u, t, c.Internal) {
				members.add(m.ID)
				notified[m.ID] = true
			}
		}
		s.notify(ctx, members, t, "mention", "mentioned "+g.Name+where)
	}

	// Everyone else involved. Internal notes never reach the requester.
	others := newRecipients(author.ID)
	for id := range notified {
		others.seen[id] = true
	}
	if t.Assignee != nil {
		others.add(t.Assignee.ID)
	}
	if c.Internal {
		s.notify(ctx, others, t, "note", "added an internal note")
		return
	}
	others.add(t.Requester.ID)
	s.notify(ctx, others, t, "reply", "replied")
}

var statusNames = map[string]string{
	"unassigned": "Unassigned", "in_progress": "In progress", "pending": "Pending",
	"resolved": "Resolved", "closed": "Closed",
}
var priorityNames = map[string]string{"low": "Low", "normal": "Normal", "high": "High", "urgent": "Urgent"}

// describeChanges summarises an update, e.g. "changed the status to Resolved"
// or "updated the status and priority".
func describeChanges(changes []store.TicketChange) string {
	if len(changes) == 1 {
		c := changes[0]
		switch c.Field {
		case "status":
			return "changed the status to " + statusNames[c.To]
		case "priority":
			return "changed the priority to " + priorityNames[c.To]
		case "category":
			if c.To == "" {
				return "removed the category"
			}
			return "set the category to " + c.To
		case "assignee":
			if c.To == "" {
				return "unassigned this ticket"
			}
			return "assigned this to " + c.To
		}
	}
	names := make([]string, len(changes))
	for i, c := range changes {
		names[i] = c.Field
	}
	if len(names) > 1 {
		return "updated the " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
	return "updated this ticket"
}

// notifyUpdate tells the requester and assignee about changes to a ticket.
// A newly assigned staff member gets "assigned this to you".
func (s *Server) notifyUpdate(ctx context.Context, t store.Ticket, actor store.User, changes []store.TicketChange) {
	if len(changes) == 0 {
		return
	}
	reassigned := false
	for _, c := range changes {
		if c.Field == "assignee" && c.To != "" {
			reassigned = true
		}
	}
	if reassigned && t.Assignee != nil {
		assignee := newRecipients(actor.ID)
		assignee.add(t.Assignee.ID)
		s.notify(ctx, assignee, t, "assigned", "assigned this to you")
	}

	others := newRecipients(actor.ID)
	others.add(t.Requester.ID)
	if t.Assignee != nil && !reassigned {
		others.add(t.Assignee.ID)
	}
	s.notify(ctx, others, t, "update", describeChanges(changes))
}
