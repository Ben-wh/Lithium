package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Attachment struct {
	ID          int64     `json:"id"`
	TicketID    int64     `json:"-"`
	Filename    string    `json:"filename"`
	StoredName  string    `json:"-"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"createdAt"`
}

// NewAttachment describes a file already written to disk, to be recorded
// alongside a new ticket.
type NewAttachment struct {
	Filename    string
	StoredName  string
	ContentType string
	Size        int64
}

const attachmentCols = `id, ticket_id, filename, stored_name, content_type, size, created_at`

func scanAttachment(row scanner) (Attachment, error) {
	var a Attachment
	var created int64
	err := row.Scan(&a.ID, &a.TicketID, &a.Filename, &a.StoredName, &a.ContentType, &a.Size, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	a.CreatedAt = time.Unix(created, 0).UTC()
	return a, err
}

func (s *Store) AttachmentByID(ctx context.Context, id int64) (Attachment, error) {
	return scanAttachment(s.db.QueryRowContext(ctx, `SELECT `+attachmentCols+` FROM ticket_attachments WHERE id = ?`, id))
}

func (s *Store) AttachmentsForTicket(ctx context.Context, ticketID int64) ([]Attachment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+attachmentCols+` FROM ticket_attachments WHERE ticket_id = ? ORDER BY id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
