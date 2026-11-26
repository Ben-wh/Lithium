package store

import (
	"testing"
	"time"
)

func mins(n int) *int { return &n }

func TestMatchSLARule(t *testing.T) {
	hw := &CategoryRef{ID: 1, Name: "Hardware"}
	// Most specific first, as SLARules returns them.
	rules := []SLARule{
		{ID: 1, Category: hw, Priority: "urgent", ResolutionMinutes: mins(60)},
		{ID: 2, Category: hw, ResolutionMinutes: mins(480)},
		{ID: 3, Priority: "urgent", ResolutionMinutes: mins(240)},
		{ID: 4, ResolutionMinutes: mins(2880)},
	}
	for _, tc := range []struct {
		cat      int64
		priority string
		want     int64
	}{
		{1, "urgent", 1}, // category and priority
		{1, "low", 2},    // category only
		{2, "urgent", 3}, // priority only
		{0, "urgent", 3},
		{2, "low", 4}, // catch-all
	} {
		got := MatchSLARule(rules, tc.cat, tc.priority)
		if got == nil || got.ID != tc.want {
			t.Errorf("cat %d %s: got %+v, want rule %d", tc.cat, tc.priority, got, tc.want)
		}
	}
	if MatchSLARule(rules[:3], 2, "low") != nil {
		t.Error("no rule should match without a catch-all")
	}
}

func TestComputeSLA(t *testing.T) {
	created := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	rules := []SLARule{{ID: 7, FirstResponseMinutes: mins(60), ResolutionMinutes: mins(240)}}
	at := func(h, m int) time.Time { return created.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }
	ptr := func(t time.Time) *time.Time { return &t }
	secs := func(n int64) *int64 { return &n }

	// Running: due times count from creation.
	st := ComputeSLA(rules, 0, "normal", "unassigned", SLAClock{CreatedAt: created}, at(0, 30))
	if st.FirstResponse.State != "running" || !st.FirstResponse.DueAt.Equal(at(1, 0)) || st.Resolution.State != "running" {
		t.Fatalf("running: %+v %+v", st.FirstResponse, st.Resolution)
	}
	// Past the first-response target, still open.
	st = ComputeSLA(rules, 0, "normal", "in_progress", SLAClock{CreatedAt: created}, at(1, 30))
	if st.FirstResponse.State != "overdue" || st.Resolution.State != "running" {
		t.Fatalf("overdue: %+v", st.FirstResponse)
	}
	// Replied after 45 minutes: met, whatever happens next.
	st = ComputeSLA(rules, 0, "normal", "in_progress", SLAClock{CreatedAt: created, FirstResponseSeconds: secs(45 * 60)}, at(9, 0))
	if st.FirstResponse.State != "met" || st.Resolution.State != "overdue" {
		t.Fatalf("met/overdue: %+v %+v", st.FirstResponse, st.Resolution)
	}
	// Pending since 1h: the clock stopped at 1h, so 3h later it's still 1h in.
	clock := SLAClock{CreatedAt: created, PausedSince: ptr(at(1, 0)), FirstResponseSeconds: secs(600)}
	st = ComputeSLA(rules, 0, "normal", "pending", clock, at(4, 0))
	if st.Resolution.State != "paused" || st.Resolution.ElapsedSeconds != 3600 {
		t.Fatalf("paused: %+v", st.Resolution)
	}
	// Back in progress after 3h paused: 1h counted before plus 30m since.
	clock = SLAClock{CreatedAt: created, PausedSeconds: 3 * 3600, FirstResponseSeconds: secs(600)}
	st = ComputeSLA(rules, 0, "normal", "in_progress", clock, at(4, 30))
	if st.Resolution.ElapsedSeconds != 90*60 || !st.Resolution.DueAt.Equal(at(7, 0)) {
		t.Fatalf("resumed: %+v", st.Resolution)
	}
	// Resolved at 3h on the clock: met; at 5h: missed.
	st = ComputeSLA(rules, 0, "normal", "resolved", SLAClock{CreatedAt: created, PausedSince: ptr(at(3, 0)), FirstResponseSeconds: secs(600)}, at(48, 0))
	if st.Resolution.State != "met" {
		t.Fatalf("met: %+v", st.Resolution)
	}
	st = ComputeSLA(rules, 0, "normal", "closed", SLAClock{CreatedAt: created, PausedSince: ptr(at(5, 0)), FirstResponseSeconds: secs(600)}, at(48, 0))
	if st.Resolution.State != "missed" {
		t.Fatalf("missed: %+v", st.Resolution)
	}
	// Finished with no reply: no first-response result.
	st = ComputeSLA(rules, 0, "normal", "resolved", SLAClock{CreatedAt: created, PausedSince: ptr(at(2, 0))}, at(3, 0))
	if st.FirstResponse != nil {
		t.Fatalf("finished without reply: %+v", st.FirstResponse)
	}
	// Raised in error, or no matching rule: no SLA at all.
	if ComputeSLA(rules, 0, "normal", "raised_in_error", SLAClock{CreatedAt: created}, at(1, 0)) != nil {
		t.Fatal("raised in error has an SLA")
	}
	if ComputeSLA(nil, 0, "normal", "unassigned", SLAClock{CreatedAt: created}, at(1, 0)) != nil {
		t.Fatal("no rules but an SLA")
	}
}
