package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lithium/lithium/internal/store"
)

// viewerLocation reads ?tz= (minutes behind UTC, as JavaScript's
// getTimezoneOffset gives it) so days line up with the viewer's calendar.
func viewerLocation(r *http.Request) *time.Location {
	offset, err := strconv.Atoi(r.URL.Query().Get("tz"))
	if err != nil || offset < -14*60 || offset > 14*60 {
		return time.UTC
	}
	return time.FixedZone("viewer", -offset*60)
}

// scopeFor applies the viewer's ticket visibility to a filter.
func (s *Server) scopeFor(w http.ResponseWriter, r *http.Request) (store.TicketFilter, bool) {
	var f store.TicketFilter
	a, err := s.accessFor(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return f, false
	}
	a.scope(&f)
	return f, true
}

// handleReport returns the Reports page figures (staff only).
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	days := 30
	switch r.URL.Query().Get("days") {
	case "7":
		days = 7
	case "90":
		days = 90
	}
	scope, ok := s.scopeFor(w, r)
	if !ok {
		return
	}
	rep, err := s.store.BuildReport(r.Context(), scope, days, viewerLocation(r))
	if err != nil {
		s.log.Error("build report", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// csvSafe stops spreadsheet apps treating a cell as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// handleExport downloads tickets raised in a date range as CSV or JSON
// (staff only, limited to what the viewer can see).
//
// Query: from, to (YYYY-MM-DD, inclusive), status (comma-separated),
// category (id), description=1, format=csv|json, tz.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	loc := viewerLocation(r)
	parse := func(name string) (time.Time, bool) {
		t, err := time.ParseInLocation("2006-01-02", qs.Get(name), loc)
		return t, err == nil
	}
	from, ok1 := parse("from")
	to, ok2 := parse("to")
	if !ok1 || !ok2 || to.Before(from) {
		writeError(w, http.StatusBadRequest, "from and to must be dates (YYYY-MM-DD), with to on or after from")
		return
	}
	f, ok := s.scopeFor(w, r)
	if !ok {
		return
	}
	if v := qs.Get("status"); v != "" {
		for _, st := range strings.Split(v, ",") {
			if !validStatuses[st] {
				writeError(w, http.StatusBadRequest, "unknown status "+strconv.Quote(st))
				return
			}
			f.Statuses = append(f.Statuses, st)
		}
	}
	if v := qs.Get("category"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "category must be a category id")
			return
		}
		f.CategoryID = id
	}
	format := qs.Get("format")
	if format != "json" {
		format = "csv"
	}
	withDesc := qs.Get("description") == "1"

	rows, err := s.store.ExportTickets(r.Context(), f, from, to.AddDate(0, 0, 1), withDesc)
	if err != nil {
		s.log.Error("export tickets", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	ref := func(id int64) string { return settings.TicketPrefix + "-" + strconv.FormatInt(id, 10) }
	name := fmt.Sprintf("lithium-tickets-%s-to-%s.%s", from.Format("2006-01-02"), to.Format("2006-01-02"), format)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	s.log.Info("tickets exported", "rows", len(rows), "format", format, "by", userFrom(r.Context()).ID)

	if format == "json" {
		type jsonRow struct {
			Reference string `json:"reference"`
			store.ExportTicket
		}
		out := make([]jsonRow, len(rows))
		for i, row := range rows {
			out[i] = jsonRow{ref(row.ID), row}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Write([]byte("\ufeff")) // BOM so Excel reads UTF-8 correctly
	cw := csv.NewWriter(w)
	header := []string{"Reference", "Subject", "Status", "Priority", "Category", "Requester", "Requester email", "Assignee", "Created (UTC)", "Updated (UTC)"}
	if withDesc {
		header = append(header, "Description")
	}
	cw.Write(header)
	for _, row := range rows {
		rec := []string{
			ref(row.ID), csvSafe(row.Subject), statusNames[row.Status], priorityNames[row.Priority], csvSafe(row.Category),
			csvSafe(row.Requester), csvSafe(row.RequesterEmail), csvSafe(row.Assignee),
			row.CreatedAt.Format("2006-01-02 15:04"), row.UpdatedAt.Format("2006-01-02 15:04"),
		}
		if withDesc {
			rec = append(rec, csvSafe(row.Description))
		}
		cw.Write(rec)
	}
	cw.Flush()
}
