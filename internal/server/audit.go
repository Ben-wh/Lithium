package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lithium/lithium/internal/store"
)

// The activity log records every change made through the API (and data
// exports), so admins can see who did what. It's driven by the route table
// below rather than calls sprinkled through the handlers, so a new write
// endpoint only needs a line here. Request bodies are never stored: only the
// fields listed for a route, and never anything that looks like a secret.

type auditRoute struct {
	action   string // stable code, e.g. "ticket.update"
	category string
	summary  string
	// target is what the {id} in the path (or the new object's id, for
	// creates) refers to: ticket, user, category, group, kbCollection, kbArticle.
	target string
	// fields are request body fields worth showing, "*" for all of them
	// (minus anything secret-looking). Nil means none.
	fields []string
	// create: the target's id comes from the response.
	create bool
	// failSummary replaces summary when the request fails, where the
	// summary would read wrongly ("Signed in" for a failed sign-in).
	failSummary string
}

const (
	catAuth      = "auth"
	catTickets   = "tickets"
	catUsers     = "users"
	catSettings  = "settings"
	catKnowledge = "knowledge"
	catProfile   = "profile"
	catReports   = "reports"
	catSystem    = "system"
)

var auditRoutes = map[string]auditRoute{
	"POST /api/auth/login":                  {action: "auth.login", category: catAuth, summary: "Signed in", failSummary: "Tried to sign in", fields: []string{"email"}},
	"POST /api/auth/logout":                 {action: "auth.logout", category: catAuth, summary: "Signed out"},
	"POST /api/auth/password":               {action: "auth.password", category: catAuth, summary: "Changed their password"},
	"POST /api/auth/2fa/verify":             {action: "auth.2fa_verify", category: catAuth, summary: "Entered a two-factor code", failSummary: "Entered a wrong two-factor code"},
	"POST /api/auth/2fa/enable":             {action: "auth.2fa_enable", category: catAuth, summary: "Turned on two-factor"},
	"POST /api/auth/2fa/skip":               {action: "auth.2fa_skip", category: catAuth, summary: "Skipped two-factor setup and turned off mandatory two-factor"},
	"POST /api/auth/2fa/recovery-codes":     {action: "auth.2fa_codes", category: catAuth, summary: "Made new recovery codes"},
	"POST /api/auth/2fa/disable":            {action: "auth.2fa_disable", category: catAuth, summary: "Turned off two-factor"},
	"POST /api/auth/password-reset-request": {action: "auth.reset_request", category: catAuth, summary: "Asked for a password reset", fields: []string{"email"}},

	"POST /api/tickets":                    {action: "ticket.create", category: catTickets, summary: "Raised a ticket", target: "ticket", create: true, fields: []string{"priority", "categoryId"}},
	"PATCH /api/tickets/{id}":              {action: "ticket.update", category: catTickets, summary: "Updated a ticket", target: "ticket", fields: []string{"status", "priority", "categoryId", "assigneeId"}},
	"PUT /api/tickets/{id}/parent":         {action: "ticket.link", category: catTickets, summary: "Linked a ticket", target: "ticket", fields: []string{"parentId"}},
	"POST /api/tickets/{id}/comments":      {action: "ticket.comment", category: catTickets, summary: "Commented on a ticket", target: "ticket", fields: []string{"internal"}},
	"PUT /api/profile/avatar":              {action: "profile.avatar", category: catProfile, summary: "Changed their profile picture"},
	"POST /api/profile/avatar":             {action: "profile.avatar", category: catProfile, summary: "Uploaded a profile picture"},
	"PUT /api/profile/email-notifications": {action: "profile.email", category: catProfile, summary: "Changed their email preferences", fields: []string{"*"}},
	"GET /api/reports/export":              {action: "report.export", category: catReports, summary: "Exported a report"},

	"POST /api/kb/collections":          {action: "kb.collection_create", category: catKnowledge, summary: "Created a knowledge base collection", target: "kbCollection", create: true, fields: []string{"name"}},
	"PUT /api/kb/collections/{id}":      {action: "kb.collection_update", category: catKnowledge, summary: "Edited a knowledge base collection", target: "kbCollection"},
	"DELETE /api/kb/collections/{id}":   {action: "kb.collection_delete", category: catKnowledge, summary: "Deleted a knowledge base collection", target: "kbCollection"},
	"POST /api/kb/articles":             {action: "kb.article_create", category: catKnowledge, summary: "Created a knowledge base article", target: "kbArticle", create: true, fields: []string{"title"}},
	"PUT /api/kb/articles/{id}":         {action: "kb.article_update", category: catKnowledge, summary: "Edited a knowledge base article", target: "kbArticle"},
	"DELETE /api/kb/articles/{id}":      {action: "kb.article_delete", category: catKnowledge, summary: "Deleted a knowledge base article", target: "kbArticle"},
	"POST /api/kb/articles/{id}/images": {action: "kb.image", category: catKnowledge, summary: "Added an image to an article", target: "kbArticle"},

	"POST /api/admin/users":                        {action: "user.create", category: catUsers, summary: "Created a user", target: "user", create: true, fields: []string{"email", "role"}},
	"POST /api/admin/users/{id}/password":          {action: "user.password", category: catUsers, summary: "Set a user's password", target: "user"},
	"PATCH /api/admin/users/{id}":                  {action: "user.update", category: catUsers, summary: "Changed a user's role", target: "user", fields: []string{"role"}},
	"DELETE /api/admin/users/{id}":                 {action: "user.delete", category: catUsers, summary: "Deleted a user", target: "user"},
	"POST /api/admin/users/{id}/2fa/reset":         {action: "user.2fa_reset", category: catUsers, summary: "Reset a user's two-factor", target: "user"},
	"POST /api/admin/password-resets/{id}/dismiss": {action: "user.reset_dismiss", category: catUsers, summary: "Dismissed a password reset request"},

	"POST /api/admin/categories":        {action: "category.create", category: catSettings, summary: "Created a category", target: "category", create: true, fields: []string{"name"}},
	"PATCH /api/admin/categories/{id}":  {action: "category.update", category: catSettings, summary: "Edited a category", target: "category", fields: []string{"*"}},
	"DELETE /api/admin/categories/{id}": {action: "category.delete", category: catSettings, summary: "Deleted a category", target: "category"},
	"PATCH /api/admin/settings":         {action: "settings.update", category: catSettings, summary: "Changed settings", fields: []string{"*"}},
	"POST /api/admin/branding/logo":     {action: "settings.logo", category: catSettings, summary: "Uploaded a company logo"},
	"DELETE /api/admin/branding/logo":   {action: "settings.logo_delete", category: catSettings, summary: "Removed the company logo"},
	"PUT /api/admin/email":              {action: "settings.email", category: catSettings, summary: "Changed the email setup", fields: []string{"host", "port", "username", "from", "tls", "baseUrl"}},
	"DELETE /api/admin/email":           {action: "settings.email_delete", category: catSettings, summary: "Turned off email"},
	"POST /api/admin/email/test":        {action: "settings.email_test", category: catSettings, summary: "Sent a test email"},
	"POST /api/admin/lockdown":          {action: "security.lockdown_start", category: catSettings, summary: "Started lockdown mode: everyone else signed out", failSummary: "Tried to start lockdown mode"},
	"POST /api/admin/lockdown/end":      {action: "security.lockdown_end", category: catSettings, summary: "Ended lockdown mode"},
	"POST /api/admin/groups":            {action: "group.create", category: catSettings, summary: "Created a group", target: "group", create: true, fields: []string{"name"}},
	"PUT /api/admin/groups/{id}":        {action: "group.update", category: catSettings, summary: "Edited a group", target: "group"},
	"DELETE /api/admin/groups/{id}":     {action: "group.delete", category: catSettings, summary: "Deleted a group", target: "group"},
}

// auditNote travels in the request context so the auth middleware and
// handlers can say who the actor is (and add detail) for the log entry.
type auditNote struct {
	actor *store.User
	extra []string
}

type auditKeyType struct{}

var auditKey auditKeyType

func auditFrom(ctx context.Context) *auditNote {
	n, _ := ctx.Value(auditKey).(*auditNote)
	return n
}

// auditActor names who's acting, for requests that don't sign in through
// the usual middleware (login, logout).
func auditActor(ctx context.Context, u store.User) {
	if n := auditFrom(ctx); n != nil {
		n.actor = &u
	}
}

// auditDetail adds a line of detail to this request's log entry.
func auditDetail(ctx context.Context, format string, args ...any) {
	if n := auditFrom(ctx); n != nil {
		n.extra = append(n.extra, fmt.Sprintf(format, args...))
	}
}

// auditRecorder notes the status and, for creates, keeps the start of the
// response body to find the new object's id.
type auditRecorder struct {
	http.ResponseWriter
	status int
	keep   bool
	body   bytes.Buffer
}

func (r *auditRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *auditRecorder) Write(b []byte) (int, error) {
	if r.keep && r.body.Len() < 64<<10 {
		r.body.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

func (r *auditRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// auditRequests wraps the router and records an entry for each audited route.
func (s *Server) auditRequests(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)
		route, ok := auditRoutes[pattern]
		if !ok || route.action == "" {
			mux.ServeHTTP(w, r)
			return
		}
		note := &auditNote{}
		r = r.WithContext(context.WithValue(r.Context(), auditKey, note))
		fields := captureFields(r, route.fields)

		// Look the target up first, so a delete still shows what was deleted.
		id := pathID64(r.URL.Path)
		var target string
		if route.target != "" && id > 0 {
			target = s.auditTargetName(r.Context(), route.target, id)
		}

		rec := &auditRecorder{ResponseWriter: w, status: http.StatusOK, keep: route.create}
		mux.ServeHTTP(rec, r)

		failed := rec.status >= 400
		// Validation mistakes aren't interesting; refused access, failed
		// sign-ins and server errors are.
		if failed && route.category != catAuth && !strings.HasPrefix(route.action, "security.") && rec.status != http.StatusUnauthorized &&
			rec.status != http.StatusForbidden && rec.status != http.StatusTooManyRequests && rec.status < 500 {
			return
		}
		if route.create && !failed {
			var created struct {
				ID int64 `json:"id"`
			}
			if json.Unmarshal(rec.body.Bytes(), &created) == nil && created.ID > 0 {
				target = s.auditTargetName(r.Context(), route.target, created.ID)
			}
		}

		ctx := context.WithoutCancel(r.Context())
		e := store.AuditEntry{
			Action: route.action, Category: route.category, Summary: route.summary,
			Target: target, Outcome: store.AuditSuccess,
			IP: s.clientIP(r), UserAgent: truncate(r.UserAgent(), 300),
		}
		if note.actor != nil {
			e.ActorID, e.ActorName, e.ActorEmail = &note.actor.ID, note.actor.Name, note.actor.Email
		} else if email := fields["email"]; email != "" && route.category == catAuth {
			e.ActorEmail = email // sign-in attempt for an account that may not exist
		}

		var detail []string
		if failed {
			e.Outcome = store.AuditFailure
			if route.failSummary != "" {
				e.Summary = route.failSummary
			}
			detail = append(detail, failureReason(rec.status))
		}
		detail = append(detail, s.describeFields(ctx, fields, route.category == catAuth)...)
		detail = append(detail, note.extra...)
		e.Detail = truncate(strings.Join(detail, " · "), 500)

		if err := s.store.AddAudit(ctx, e); err != nil {
			s.log.Error("write activity log", "err", err, "action", e.Action)
		}
	})
}

func failureReason(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "failed: wrong details or not signed in"
	case http.StatusForbidden:
		return "failed: not allowed"
	case http.StatusTooManyRequests:
		return "failed: too many attempts"
	case http.StatusLocked:
		return "blocked: lockdown mode"
	case http.StatusBadRequest:
		return "failed: rejected"
	}
	if status >= 500 {
		return "failed: server error"
	}
	return "failed (" + strconv.Itoa(status) + ")"
}

// secretish field names are never recorded, even from a "*" route.
func secretish(key string) bool {
	k := strings.ToLower(key)
	for _, s := range []string{"password", "secret", "code", "token", "key"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// captureFields reads the wanted top-level JSON fields from the request body
// and puts the body back for the handler.
func captureFields(r *http.Request, want []string) map[string]string {
	if len(want) == 0 || r.Body == nil || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	var body map[string]json.RawMessage
	if json.Unmarshal(raw, &body) != nil {
		return nil
	}
	all := len(want) == 1 && want[0] == "*"
	out := map[string]string{}
	for k, v := range body {
		if secretish(k) || (!all && !contains(want, k)) {
			continue
		}
		out[k] = jsonValue(v)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// jsonValue turns a JSON value into short readable text.
func jsonValue(v json.RawMessage) string {
	var x any
	if json.Unmarshal(v, &x) != nil {
		return ""
	}
	switch t := x.(type) {
	case nil:
		return "none"
	case bool:
		if t {
			return "on"
		}
		return "off"
	case string:
		return truncate(t, 80)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, fmt.Sprint(e))
		}
		return truncate(strings.Join(parts, ", "), 80)
	default:
		return "…"
	}
}

// describeFields renders captured fields as "status: closed · assignee: Sam".
func (s *Server) describeFields(ctx context.Context, fields map[string]string, skipEmail bool) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if skipEmail && k == "email" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := fields[k]
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			switch k {
			case "assigneeId":
				v = s.auditTargetName(ctx, "user", id)
			case "categoryId":
				v = s.auditTargetName(ctx, "category", id)
			case "parentId":
				v = s.auditTargetName(ctx, "ticket", id)
			}
		}
		out = append(out, humanKey(k)+": "+v)
	}
	return out
}

// humanKey turns "requireTwoFactor" into "require two factor" and drops an
// "Id" suffix ("assigneeId" → "assignee").
func humanKey(k string) string {
	k = strings.TrimSuffix(k, "Id")
	var b strings.Builder
	for i, r := range k {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// auditTargetName is a readable name for what an action was done to. Tickets
// are shown by reference only, since subjects can be sensitive.
func (s *Server) auditTargetName(ctx context.Context, kind string, id int64) string {
	switch kind {
	case "ticket":
		prefix := "TID"
		if st, err := s.store.Settings(ctx); err == nil {
			prefix = st.TicketPrefix
		}
		return prefix + "-" + strconv.FormatInt(id, 10)
	case "user":
		if u, err := s.store.UserByID(ctx, id); err == nil {
			return u.Name + " (" + u.Email + ")"
		}
	case "category":
		if c, err := s.store.CategoryByID(ctx, id); err == nil {
			return c.Name
		}
	case "group":
		if g, err := s.store.GroupByID(ctx, id); err == nil {
			return g.Name
		}
	case "kbCollection":
		if c, err := s.store.KBCollectionByID(ctx, id); err == nil {
			return c.Name
		}
	case "kbArticle":
		if a, err := s.store.KBArticleByID(ctx, id); err == nil {
			return a.Title
		}
	}
	return "#" + strconv.FormatInt(id, 10)
}

// pathID64 finds the first numeric segment of an API path ("/api/tickets/42/…").
func pathID64(path string) int64 {
	for _, seg := range strings.Split(path, "/") {
		if id, err := strconv.ParseInt(seg, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Don't cut a multi-byte character in half.
	for n > 0 && !utf8Start(s[n]) {
		n--
	}
	return s[:n] + "…"
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// handleActivity lists the activity log for admins, newest first.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := store.AuditQuery{
		Search:   strings.TrimSpace(q.Get("q")),
		Category: q.Get("category"),
		Outcome:  q.Get("outcome"),
		Limit:    50,
	}
	if len(query.Search) > 200 {
		query.Search = query.Search[:200]
	}
	query.ActorID, _ = strconv.ParseInt(q.Get("actor"), 10, 64)
	query.BeforeID, _ = strconv.ParseInt(q.Get("before"), 10, 64)
	if n, err := strconv.ParseInt(q.Get("since"), 10, 64); err == nil && n > 0 {
		query.Since = time.Unix(n, 0)
	}
	if n, err := strconv.ParseInt(q.Get("until"), 10, 64); err == nil && n > 0 {
		query.Until = time.Unix(n, 0)
	}
	// One extra tells us whether there's another page.
	query.Limit++
	entries, err := s.store.AuditLog(r.Context(), query)
	if err != nil {
		s.log.Error("read activity log", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	more := len(entries) > query.Limit-1
	if more {
		entries = entries[:query.Limit-1]
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "hasMore": more})
}
