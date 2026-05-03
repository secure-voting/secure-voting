package elections

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ImportInvites(ctx context.Context, electionID, adminUserID, filename string, content []byte) (InviteImportResult, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return InviteImportResult{}, "invalid_id", nil
	}

	var accessMode string
	err := s.db.QueryRow(ctx, `
		SELECT access_mode
		FROM elections
		WHERE id = $1::uuid AND created_by = $2::uuid
	`, electionID, adminUserID).Scan(&accessMode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return InviteImportResult{}, "not_found", nil
		}
		return InviteImportResult{}, "", err
	}
	if accessMode != "invite" {
		return InviteImportResult{}, "not_invite_mode", nil
	}

	emails, code, err := parseInviteImportEmails(filename, content)
	if err != nil || code != "" {
		return InviteImportResult{}, code, err
	}

	result := InviteImportResult{
		Total:                len(emails),
		Parsed:               len(emails),
		Created:              make([]InviteImportItem, 0),
		RegistrationRequired: make([]InviteImportItem, 0),
		Skipped:              make([]InviteImportItem, 0),
		Failed:               make([]InviteImportItem, 0),
	}

	seen := make(map[string]struct{}, len(emails))

	for _, rawEmail := range emails {
		email := strings.TrimSpace(strings.ToLower(rawEmail))
		if email == "" {
			continue
		}

		if _, ok := seen[email]; ok {
			code := "duplicate_email_in_file"
			result.Skipped = append(result.Skipped, InviteImportItem{
				Email: email,
				Code:  &code,
			})
			continue
		}
		seen[email] = struct{}{}

		created, code, err := s.CreateInvite(ctx, electionID, adminUserID, email)
		if err != nil {
			return InviteImportResult{}, "", err
		}

		switch code {
		case "":
			inviteID := created.InviteID
			status := created.Status
			result.Created = append(result.Created, InviteImportItem{
				Email:    email,
				InviteID: &inviteID,
				Status:   &status,
			})
		case "registration_required":
			result.RegistrationRequired = append(result.RegistrationRequired, InviteImportItem{
				Email: email,
			})
		case "email_already_invited":
			result.Skipped = append(result.Skipped, InviteImportItem{
				Email: email,
				Code:  &code,
			})
		default:
			result.Failed = append(result.Failed, InviteImportItem{
				Email: email,
				Code:  &code,
			})
		}
	}

	return result, "", nil
}

func parseInviteImportEmails(filename string, content []byte) ([]string, string, error) {
	name := strings.TrimSpace(filename)
	if name == "" {
		return nil, "invalid_file", nil
	}

	data := stripUTF8BOM(content)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, "invalid_file", nil
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		return parseInviteEmailsCSV(data)
	case ".json":
		return parseInviteEmailsJSON(data)
	default:
		return nil, "unsupported_file_format", nil
	}
}

func parseInviteEmailsCSV(data []byte) ([]string, string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, "invalid_file", nil
	}
	if len(rows) == 0 {
		return nil, "invalid_file", nil
	}

	startRow := 0
	emailIdx := 0

	if idx, ok := inviteCSVColumnIndex(rows[0], "email", "e_mail", "mail"); ok {
		startRow = 1
		emailIdx = idx
	}

	out := make([]string, 0, len(rows))
	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		if inviteCSVRowEmpty(row) {
			continue
		}
		if emailIdx >= len(row) {
			return nil, "invalid_file", nil
		}
		email := strings.TrimSpace(row[emailIdx])
		if email == "" {
			continue
		}
		out = append(out, email)
	}

	if len(out) == 0 {
		return nil, "invalid_file", nil
	}

	return out, "", nil
}

func parseInviteEmailsJSON(data []byte) ([]string, string, error) {
	var wrapped struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Items) > 0 {
		return wrapped.Items, "", nil
	}

	var names []string
	if err := json.Unmarshal(data, &names); err == nil && len(names) > 0 {
		return names, "", nil
	}

	var objects []struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &objects); err == nil && len(objects) > 0 {
		out := make([]string, 0, len(objects))
		for _, item := range objects {
			if strings.TrimSpace(item.Email) != "" {
				out = append(out, item.Email)
			}
		}
		if len(out) > 0 {
			return out, "", nil
		}
	}

	return nil, "invalid_file", nil
}

func inviteCSVColumnIndex(header []string, names ...string) (int, bool) {
	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[normalizeInviteCSVHeader(name)] = struct{}{}
	}

	for i, col := range header {
		if _, ok := want[normalizeInviteCSVHeader(col)]; ok {
			return i, true
		}
	}

	return -1, false
}

func normalizeInviteCSVHeader(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func inviteCSVRowEmpty(row []string) bool {
	for _, col := range row {
		if strings.TrimSpace(col) != "" {
			return false
		}
	}
	return true
}
