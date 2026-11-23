package store

import (
	"context"
	"strconv"
	"time"
)

// Settings is the workspace-wide configuration. Add a field here, a key
// constant below, and a case in Settings() to introduce a new setting.
type Settings struct {
	// RequireCategory makes choosing a category mandatory when raising a
	// ticket (only enforced while at least one active category exists).
	RequireCategory bool `json:"requireCategory"`
	// AllowImageUploads lets users attach images when raising a ticket.
	AllowImageUploads bool `json:"allowImageUploads"`
	// TicketPrefix is shown before ticket numbers, e.g. "TID" → "TID-42".
	TicketPrefix string `json:"ticketPrefix"`
	// DisableCustomAvatars stops users uploading their own profile picture
	// (and hides existing ones); the built-in avatars stay available.
	DisableCustomAvatars bool `json:"disableCustomAvatars"`
	// AllowUserMentions lets regular users (not just staff) @mention people.
	AllowUserMentions bool `json:"allowUserMentions"`
	// SubjectMaxLength and DescriptionMaxLength cap new tickets, in characters.
	SubjectMaxLength     int `json:"subjectMaxLength"`
	DescriptionMaxLength int `json:"descriptionMaxLength"`
	// IncidentTree shows staff a diagram of tickets linked under an urgent ticket.
	IncidentTree bool `json:"incidentTree"`
	// AgentGroups limits agents to tickets in their groups' categories.
	AgentGroups bool `json:"agentGroups"`
	// Background is the app-wide background pattern (see Backgrounds).
	Background string `json:"background"`
	// SensitiveMode shows a red banner with SensitiveMessage above the navbar
	// on every page.
	SensitiveMode    bool   `json:"sensitiveMode"`
	SensitiveMessage string `json:"sensitiveMessage"`
	// RequireTwoFactor makes everyone set up an authenticator app before
	// they can use Lithium.
	RequireTwoFactor bool `json:"requireTwoFactor"`
}

// DefaultSensitiveMessage is shown when sensitive mode has no custom message.
const DefaultSensitiveMessage = "Sensitive system: handle all data in line with policy"

// Backgrounds admins can choose from. Keep in sync with web/src/lib/backgrounds.ts.
var Backgrounds = map[string]bool{"none": true, "dots": true, "grid": true, "lines": true, "waves": true, "glow": true}

// Allowed ranges for the ticket length limits. The upper bounds are what the
// server's request size limits are built around.
const (
	SubjectMaxLengthMin     = 20
	SubjectMaxLengthMax     = 200
	DescriptionMaxLengthMin = 100
	DescriptionMaxLengthMax = 20000
)

const (
	keyRequireCategory      = "tickets.require_category"
	keyAllowImageUploads    = "tickets.allow_image_uploads"
	keyTicketPrefix         = "tickets.id_prefix"
	keyDisableCustomAvatars = "users.disable_custom_avatars"
	keyAllowUserMentions    = "users.allow_user_mentions"
	keySubjectMaxLength     = "tickets.subject_max_length"
	keyDescriptionMaxLength = "tickets.description_max_length"
	keyIncidentTree         = "tickets.incident_tree"
	keyAgentGroups          = "users.agent_groups"
	keyBackground           = "appearance.background"
	keySensitiveMode        = "appearance.sensitive_mode"
	keySensitiveMessage     = "appearance.sensitive_message"
	keyRequireTwoFactor     = "users.require_two_factor"
)

// DefaultSettings are used for any setting that hasn't been saved.
func DefaultSettings() Settings {
	return Settings{
		RequireCategory:      false,
		AllowImageUploads:    true,
		TicketPrefix:         "TID",
		AllowUserMentions:    true,
		SubjectMaxLength:     150,  // about one line, like an email subject
		DescriptionMaxLength: 5000, // about a page of text
		IncidentTree:         true,
		AgentGroups:          false,
		Background:           "none",
		SensitiveMode:        false,
		SensitiveMessage:     DefaultSensitiveMessage,
		RequireTwoFactor:     true,
	}
}

func (s *Store) Settings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return st, err
		}
		switch key {
		case keyRequireCategory:
			st.RequireCategory, _ = strconv.ParseBool(value)
		case keyAllowImageUploads:
			st.AllowImageUploads, _ = strconv.ParseBool(value)
		case keyTicketPrefix:
			st.TicketPrefix = value
		case keyDisableCustomAvatars:
			st.DisableCustomAvatars, _ = strconv.ParseBool(value)
		case keyAllowUserMentions:
			st.AllowUserMentions, _ = strconv.ParseBool(value)
		case keySubjectMaxLength:
			if n, err := strconv.Atoi(value); err == nil {
				st.SubjectMaxLength = n
			}
		case keySensitiveMode:
			st.SensitiveMode, _ = strconv.ParseBool(value)
		case keySensitiveMessage:
			if value != "" {
				st.SensitiveMessage = value
			}
		case keyBackground:
			if Backgrounds[value] {
				st.Background = value
			}
		case keyRequireTwoFactor:
			st.RequireTwoFactor, _ = strconv.ParseBool(value)
		case keyAgentGroups:
			st.AgentGroups, _ = strconv.ParseBool(value)
		case keyIncidentTree:
			st.IncidentTree, _ = strconv.ParseBool(value)
		case keyDescriptionMaxLength:
			if n, err := strconv.Atoi(value); err == nil {
				st.DescriptionMaxLength = n
			}
		}
	}
	return st, rows.Err()
}

// SettingsUpdate holds the settings to change; nil fields are left alone.
type SettingsUpdate struct {
	RequireCategory      *bool
	AllowImageUploads    *bool
	TicketPrefix         *string
	DisableCustomAvatars *bool
	AllowUserMentions    *bool
	SubjectMaxLength     *int
	DescriptionMaxLength *int
	IncidentTree         *bool
	AgentGroups          *bool
	Background           *string
	SensitiveMode        *bool
	SensitiveMessage     *string
	RequireTwoFactor     *bool
}

func (s *Store) UpdateSettings(ctx context.Context, u SettingsUpdate) (Settings, error) {
	values := map[string]string{}
	if u.RequireCategory != nil {
		values[keyRequireCategory] = strconv.FormatBool(*u.RequireCategory)
	}
	if u.AllowImageUploads != nil {
		values[keyAllowImageUploads] = strconv.FormatBool(*u.AllowImageUploads)
	}
	if u.TicketPrefix != nil {
		values[keyTicketPrefix] = *u.TicketPrefix
	}
	if u.DisableCustomAvatars != nil {
		values[keyDisableCustomAvatars] = strconv.FormatBool(*u.DisableCustomAvatars)
	}
	if u.AllowUserMentions != nil {
		values[keyAllowUserMentions] = strconv.FormatBool(*u.AllowUserMentions)
	}
	if u.SubjectMaxLength != nil {
		values[keySubjectMaxLength] = strconv.Itoa(*u.SubjectMaxLength)
	}
	if u.SensitiveMode != nil {
		values[keySensitiveMode] = strconv.FormatBool(*u.SensitiveMode)
	}
	if u.SensitiveMessage != nil {
		values[keySensitiveMessage] = *u.SensitiveMessage
	}
	if u.Background != nil {
		values[keyBackground] = *u.Background
	}
	if u.RequireTwoFactor != nil {
		values[keyRequireTwoFactor] = strconv.FormatBool(*u.RequireTwoFactor)
	}
	if u.AgentGroups != nil {
		values[keyAgentGroups] = strconv.FormatBool(*u.AgentGroups)
	}
	if u.IncidentTree != nil {
		values[keyIncidentTree] = strconv.FormatBool(*u.IncidentTree)
	}
	if u.DescriptionMaxLength != nil {
		values[keyDescriptionMaxLength] = strconv.Itoa(*u.DescriptionMaxLength)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	for k, v := range values {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			k, v, now); err != nil {
			return Settings{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, err
	}
	return s.Settings(ctx)
}

// HasActiveCategories reports whether any non-archived category exists.
func (s *Store) HasActiveCategories(ctx context.Context) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ticket_categories WHERE archived = 0)`).Scan(&ok)
	return ok, err
}
