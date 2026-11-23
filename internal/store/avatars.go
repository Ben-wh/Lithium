package store

import (
	"context"
	"fmt"
	"strings"
)

// PresetAvatarCount is how many built-in avatars the web app ships
// (web/static/avatars/1.svg … 10.svg).
const PresetAvatarCount = 10

// AvatarURL turns a stored avatar value into the URL the web app loads.
// Custom pictures are served by the API, which hides them when an admin has
// disabled custom avatars; the app then falls back to initials.
func AvatarURL(userID int64, avatar string) string {
	switch {
	case strings.HasPrefix(avatar, "preset:"):
		return "/avatars/" + strings.TrimPrefix(avatar, "preset:") + ".svg"
	case strings.HasPrefix(avatar, "custom:"):
		name := strings.TrimPrefix(avatar, "custom:")
		v := name
		if len(v) > 8 {
			v = v[:8]
		}
		// The version changes with every upload, so browsers never show a stale picture.
		return fmt.Sprintf("/api/avatars/%d?v=%s", userID, v)
	}
	return ""
}

// CustomAvatarFile returns the stored file name of a custom avatar, or "".
func CustomAvatarFile(avatar string) string {
	if name, ok := strings.CutPrefix(avatar, "custom:"); ok {
		return name
	}
	return ""
}

// SetAvatar stores a user's avatar and returns the previous value, so the
// caller can delete a replaced custom file.
func (s *Store) SetAvatar(ctx context.Context, userID int64, avatar string) (previous string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT avatar FROM users WHERE id = ?`, userID).Scan(&previous); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET avatar = ? WHERE id = ?`, avatar, userID); err != nil {
		return "", err
	}
	return previous, tx.Commit()
}
