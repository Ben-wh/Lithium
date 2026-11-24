package mail

import (
	"context"
	"log/slog"
	"time"

	"github.com/lithium/lithium/internal/store"
)

// Retry delays after each failed attempt. After the last one the email is
// given up on (it stays in the outbox with its error, for a while).
var retryDelays = []time.Duration{
	time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 4 * time.Hour, 12 * time.Hour,
}

const (
	pollInterval = 10 * time.Second
	batchSize    = 20
	keepFor      = 30 * 24 * time.Hour // how long sent and abandoned emails are kept
)

// Outbox works through queued emails in the background.
type Outbox struct {
	Store    *store.Store
	Sender   Sender
	From     string
	FromName func(ctx context.Context) string // looked up per batch, so branding changes apply
	Log      *slog.Logger
	Now      func() time.Time // for tests; time.Now when nil
}

func (o *Outbox) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Run sends due emails every few seconds until ctx is cancelled.
func (o *Outbox) Run(ctx context.Context) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	lastPrune := time.Time{}
	for {
		o.SendDue(ctx)
		if time.Since(lastPrune) > time.Hour {
			if n, err := o.Store.PruneEmails(ctx, o.now().Add(-keepFor)); err != nil && ctx.Err() == nil {
				o.Log.Error("prune email outbox", "err", err)
			} else if n > 0 {
				o.Log.Info("pruned email outbox", "count", n)
			}
			lastPrune = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// SendDue sends every email that's due now, one batch at a time.
func (o *Outbox) SendDue(ctx context.Context) {
	for ctx.Err() == nil {
		due, err := o.Store.DueEmails(ctx, o.now(), batchSize)
		if err != nil {
			if ctx.Err() == nil {
				o.Log.Error("load email outbox", "err", err)
			}
			return
		}
		if len(due) == 0 {
			return
		}
		fromName := o.FromName(ctx)
		for _, e := range due {
			o.send(ctx, e, fromName)
		}
		if len(due) < batchSize {
			return
		}
	}
}

func (o *Outbox) send(ctx context.Context, e store.OutboxEmail, fromName string) {
	sendCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	err := o.Sender.Send(sendCtx, Message{FromName: fromName, From: o.From, To: e.To, Subject: e.Subject, Body: e.Body})
	cancel()
	if ctx.Err() != nil {
		return // shutting down; it'll be retried next start
	}
	if err == nil {
		if err := o.Store.MarkEmailSent(ctx, e.ID, o.now()); err != nil {
			o.Log.Error("mark email sent", "err", err, "email_id", e.ID)
		}
		return
	}

	var retryAt time.Time
	if e.Attempts < len(retryDelays) {
		retryAt = o.now().Add(retryDelays[e.Attempts])
		o.Log.Warn("send email failed; will retry", "err", err, "email_id", e.ID, "attempt", e.Attempts+1, "retry_at", retryAt)
	} else {
		o.Log.Error("send email failed; giving up", "err", err, "email_id", e.ID, "attempts", e.Attempts+1)
	}
	if err := o.Store.MarkEmailFailed(ctx, e.ID, err.Error(), retryAt); err != nil {
		o.Log.Error("mark email failed", "err", err, "email_id", e.ID)
	}
}
