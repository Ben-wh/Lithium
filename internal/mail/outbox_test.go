package mail

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/db"
	"github.com/lithium/lithium/internal/store"
)

type fakeSender struct {
	sent []Message
	fail error
}

func (f *fakeSender) Send(_ context.Context, m Message) error {
	if f.fail != nil {
		return f.fail
	}
	f.sent = append(f.sent, m)
	return nil
}

// newOutbox returns an outbox with one queued email to alice, and a clock
// the test can move.
func newOutbox(t *testing.T) (*Outbox, *fakeSender, *time.Time) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	st := store.New(conn)
	u, err := st.CreateUser(ctx, "alice@example.com", "Alice", "x", "user", false)
	if err != nil {
		t.Fatal(err)
	}
	tk, err := st.CreateTicket(ctx, store.NewTicket{Subject: "S", Description: "d", RequesterID: u.ID, CreatedBy: u.ID, Priority: "normal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Notify(ctx, []int64{u.ID}, tk.ID, 0, "reply", "replied", &store.EmailContent{Subject: "Hello", Line: "Alice replied.", Rest: "Body"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(store.EmailBatchWindow) // once the batch window has passed
	f := &fakeSender{}
	return &Outbox{
		Store: st, Sender: f, From: "help@example.com",
		FromName: func(context.Context) string { return "Acme" },
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      func() time.Time { return now },
	}, f, &now
}

func TestOutboxSends(t *testing.T) {
	o, f, _ := newOutbox(t)
	o.SendDue(context.Background())
	if len(f.sent) != 1 || f.sent[0].To != "alice@example.com" || f.sent[0].FromName != "Acme" || f.sent[0].Subject != "Hello" || f.sent[0].Body != "Alice replied.\n\nBody" {
		t.Fatalf("sent: %+v", f.sent)
	}
	o.SendDue(context.Background())
	if len(f.sent) != 1 {
		t.Fatal("sent twice")
	}
}

func TestOutboxRetriesThenGivesUp(t *testing.T) {
	o, f, now := newOutbox(t)
	ctx := context.Background()
	f.fail = errors.New("connection refused")

	for i, delay := range retryDelays {
		o.SendDue(ctx)
		due, _ := o.Store.DueEmails(ctx, *now, 10)
		if len(due) != 0 {
			t.Fatalf("attempt %d: retried immediately", i+1)
		}
		*now = now.Add(delay)
		if due, _ = o.Store.DueEmails(ctx, *now, 10); len(due) != 1 {
			t.Fatalf("attempt %d: not due after %v", i+1, delay)
		}
	}
	o.SendDue(ctx) // the last attempt fails too: give up
	*now = now.Add(365 * 24 * time.Hour)
	if due, _ := o.Store.DueEmails(ctx, *now, 10); len(due) != 0 {
		t.Fatal("should have given up")
	}
	if n, _ := o.Store.PruneEmails(ctx, *now); n != 1 {
		t.Fatalf("pruned %d", n)
	}
}

func TestOutboxRecoversAfterFailure(t *testing.T) {
	o, f, now := newOutbox(t)
	f.fail = errors.New("temporary")
	o.SendDue(context.Background())
	f.fail = nil
	*now = now.Add(retryDelays[0])
	o.SendDue(context.Background())
	if len(f.sent) != 1 {
		t.Fatalf("not sent after recovery: %+v", f.sent)
	}
}
