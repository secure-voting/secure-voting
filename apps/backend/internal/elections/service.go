package elections

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

var allowedBallotFormats = map[string]bool{
	"approval": true,
	"ranking":  true,
	"score":    true,
}

var allowedAccessModes = map[string]bool{
	"open":   true,
	"invite": true,
}

var allowedQuotaTypes = map[string]bool{
	"hare":  true,
	"droop": true,
}

// Под твой Rust-core (и под ТЗ). Можно расширять без миграций.
var allowedTallyRules = map[string]bool{
	"plurality":            true,
	"approval":             true,
	"inverse_plurality":    true,
	"borda":                true,
	"black":                true,
	"copeland_i":           true,
	"copeland_ii":          true,
	"copeland_iii":         true,
	"simpson":              true,
	"minmax":               true,
	"minimax":              true,
	"hare":                 true,
	"inverse_borda":        true,
	"nanson":               true,
	"coombs":               true,
	"practical_condorcet":  true,
	"condorcet_practical":  true,
	"threshold":            true,
}

func validateTallyRule(v string) (string, bool) {
	n := norm(v)
	if n == "" {
		return "", false
	}
	if !allowedTallyRules[n] {
		return "", false
	}
	return n, true
}

func validateBallotParams(format string, candidatesCount int, approvalMaxChoices *int, rankingTopK *int, scoreMin *int, scoreMax *int, scoreStep *int) string {
	if candidatesCount <= 0 {
		return "candidates_required"
	}

	switch format {
	case "approval":
		if approvalMaxChoices == nil {
			return "approval_max_choices_required"
		}
		if *approvalMaxChoices <= 0 {
			return "invalid_approval_max_choices"
		}
		if *approvalMaxChoices > candidatesCount {
			return "approval_max_choices_too_large"
		}
		return ""
	case "ranking":
		if rankingTopK == nil {
			return "ranking_top_k_required"
		}
		if *rankingTopK <= 0 {
			return "invalid_ranking_top_k"
		}
		if *rankingTopK > candidatesCount {
			return "ranking_top_k_too_large"
		}
		return ""
	case "score":
		if scoreMin == nil || scoreMax == nil || scoreStep == nil {
			return "score_range_required"
		}
		if *scoreStep <= 0 {
			return "invalid_score_step"
		}
		if *scoreMin > *scoreMax {
			return "invalid_score_range"
		}
		if (*scoreMax-*scoreMin)%*scoreStep != 0 {
			return "invalid_score_step_range"
		}
		return ""
	default:
		return "invalid_ballot_format"
	}
}

type CandidateInput struct {
	Name string         `json:"name"`
	Meta map[string]any `json:"meta,omitempty"`
}

type CreateElectionInput struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`

	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`

	TallyRule    string `json:"tally_rule"`
	BallotFormat string `json:"ballot_format"`

	CommitteeSize *int    `json:"committee_size,omitempty"`
	QuotaType     *string `json:"quota_type,omitempty"`

	AccessMode     string  `json:"access_mode"`
	PublishAt      *string `json:"publish_at,omitempty"`
	ShowAggregates bool    `json:"show_aggregates"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`

	ScoreMin       *int `json:"score_min,omitempty"`
	ScoreMax       *int `json:"score_max,omitempty"`
	ScoreStep      *int `json:"score_step,omitempty"`
	ScoreAllowSkip bool `json:"score_allow_skip"`

	Candidates []CandidateInput `json:"candidates"`
}

type ElectionSummary struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	AccessMode  string  `json:"access_mode"`
	StartAt     string  `json:"start_at"`
	EndAt       string  `json:"end_at"`
	PublishedAt *string `json:"published_at,omitempty"`
}

type Candidate struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Meta map[string]any `json:"meta,omitempty"`
}

type BallotMeta struct {
	ElectionID string `json:"election_id"`

	TallyRule    string `json:"tally_rule"`
	BallotFormat string `json:"ballot_format"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`

	ScoreMin       *int `json:"score_min,omitempty"`
	ScoreMax       *int `json:"score_max,omitempty"`
	ScoreStep      *int `json:"score_step,omitempty"`
	ScoreAllowSkip bool `json:"score_allow_skip"`

	Candidates []Candidate `json:"candidates"`
}

type UpdateRulesInput struct {
	TallyRule    *string `json:"tally_rule,omitempty"`
	BallotFormat *string `json:"ballot_format,omitempty"`

	CommitteeSize *int    `json:"committee_size,omitempty"`
	QuotaType     *string `json:"quota_type,omitempty"`

	AccessMode     *string `json:"access_mode,omitempty"`
	PublishAt      *string `json:"publish_at,omitempty"`
	ShowAggregates *bool   `json:"show_aggregates,omitempty"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`

	ScoreMin       *int  `json:"score_min,omitempty"`
	ScoreMax       *int  `json:"score_max,omitempty"`
	ScoreStep      *int  `json:"score_step,omitempty"`
	ScoreAllowSkip *bool `json:"score_allow_skip,omitempty"`
}

type Invite struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	Status     string  `json:"status"`
	SentAt     *string `json:"sent_at,omitempty"`
	AcceptedAt *string `json:"accepted_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type InviteCreated struct {
	InviteID   string `json:"invite_id"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

func (s *Service) Create(ctx context.Context, createdBy string, in CreateElectionInput) (string, string, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return "", "invalid_title", nil
	}

	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.StartAt))
	if err != nil {
		return "", "invalid_start_at", nil
	}
	endAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.EndAt))
	if err != nil {
		return "", "invalid_end_at", nil
	}
	if !startAt.Before(endAt) {
		return "", "invalid_time_range", nil
	}

	tally, ok := validateTallyRule(in.TallyRule)
	if !ok {
		return "", "invalid_tally_rule", nil
	}
	format := norm(in.BallotFormat)
	if !allowedBallotFormats[format] {
		return "", "invalid_ballot_format", nil
	}
	access := norm(in.AccessMode)
	if !allowedAccessModes[access] {
		return "", "invalid_access_mode", nil
	}

	// кандидаты
	if len(in.Candidates) == 0 {
		return "", "candidates_required", nil
	}
	seen := make(map[string]struct{}, len(in.Candidates))
	for _, c := range in.Candidates {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			return "", "invalid_candidate_name", nil
		}
		key := norm(name)
		if _, exists := seen[key]; exists {
			return "", "duplicate_candidate_name", nil
		}
		seen[key] = struct{}{}
	}

	// committee/quota (минимально безопасно)
	if in.CommitteeSize != nil && *in.CommitteeSize <= 0 {
		return "", "invalid_committee_size", nil
	}
	if in.CommitteeSize != nil && *in.CommitteeSize > 1 {
		if in.QuotaType == nil {
			return "", "quota_type_required", nil
		}
		qt := norm(*in.QuotaType)
		if !allowedQuotaTypes[qt] {
			return "", "invalid_quota_type", nil
		}
		in.QuotaType = &qt
	} else {
		// single-winner: quota_type не нужен
		in.QuotaType = nil
	}

	// publish_at
	var publishAt *time.Time
	if in.PublishAt != nil {
		p := strings.TrimSpace(*in.PublishAt)
		if p != "" {
			t, err := time.Parse(time.RFC3339, p)
			if err != nil {
				return "", "invalid_publish_at", nil
			}
			publishAt = &t
		} else {
			publishAt = nil
		}
	}

	// ballot params
	code := validateBallotParams(format, len(in.Candidates), in.ApprovalMaxChoices, in.RankingTopK, in.ScoreMin, in.ScoreMax, in.ScoreStep)
	if code != "" {
		return "", code, nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var electionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO elections (
			title, description, start_at, end_at, tally_rule, ballot_format,
			committee_size, quota_type,
			status, access_mode,
			publish_at, show_aggregates,
			approval_max_choices, ranking_top_k,
			score_min, score_max, score_step, score_allow_skip,
			created_by
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,
			'draft',$9,
			$10,$11,
			$12,$13,
			$14,$15,$16,$17,
			$18
		)
		RETURNING id::text
	`, in.Title, in.Description, startAt, endAt, tally, format,
		in.CommitteeSize, in.QuotaType,
		access,
		publishAt, in.ShowAggregates,
		in.ApprovalMaxChoices, in.RankingTopK,
		in.ScoreMin, in.ScoreMax, in.ScoreStep, in.ScoreAllowSkip,
		createdBy,
	).Scan(&electionID)
	if err != nil {
		return "", "", err
	}

	for _, c := range in.Candidates {
		var metaJSON []byte
		if c.Meta != nil {
			metaJSON, err = json.Marshal(c.Meta)
			if err != nil {
				return "", "", err
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO candidates (election_id, name, meta)
			VALUES ($1::uuid, $2, $3::jsonb)
		`, electionID, strings.TrimSpace(c.Name), nullableJSON(metaJSON))
		if err != nil {
			return "", "", err
		}
	}

	_ = insertAudit(ctx, tx, &createdBy, "election_created", map[string]any{
		"target_type": "election",
		"target_id":   electionID,
		"after": map[string]any{
			"title": in.Title,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}

	return electionID, "", nil
}

func (s *Service) ListForUser(ctx context.Context, userID, email, role string) ([]ElectionSummary, error) {
	var rows pgx.Rows
	var err error

	if role == "admin" {
		rows, err = s.db.Query(ctx, `
			SELECT id::text, title, description, status, access_mode, start_at, end_at, published_at
			FROM elections
			WHERE created_by = $1::uuid
			ORDER BY created_at DESC
		`, userID)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT e.id::text, e.title, e.description, e.status, e.access_mode, e.start_at, e.end_at, e.published_at
			FROM elections e
			WHERE e.access_mode = 'open'
			   OR EXISTS (
				 SELECT 1 FROM election_invites i
				 WHERE i.election_id = e.id
				   AND lower(i.email) = lower($1)
				   AND i.status IN ('created','sent','accepted')
			   )
			ORDER BY e.created_at DESC
		`, email)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ElectionSummary
	for rows.Next() {
		var e ElectionSummary
		var startAt, endAt time.Time
		var publishedAt *time.Time
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.AccessMode, &startAt, &endAt, &publishedAt); err != nil {
			return nil, err
		}
		e.StartAt = startAt.UTC().Format(time.RFC3339)
		e.EndAt = endAt.UTC().Format(time.RFC3339)
		if publishedAt != nil {
			sv := publishedAt.UTC().Format(time.RFC3339)
			e.PublishedAt = &sv
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) GetBallotMeta(ctx context.Context, electionID, userID, email, role string) (BallotMeta, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return BallotMeta{}, "invalid_id", nil
	}

	allowed, err := s.isAccessible(ctx, electionID, userID, email, role)
	if err != nil {
		return BallotMeta{}, "", err
	}
	if !allowed {
		return BallotMeta{}, "not_found", nil
	}

	var meta BallotMeta
	meta.ElectionID = electionID

	err = s.db.QueryRow(ctx, `
		SELECT tally_rule, ballot_format,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip
		FROM elections
		WHERE id=$1::uuid
	`, electionID).Scan(
		&meta.TallyRule, &meta.BallotFormat,
		&meta.ApprovalMaxChoices, &meta.RankingTopK,
		&meta.ScoreMin, &meta.ScoreMax, &meta.ScoreStep, &meta.ScoreAllowSkip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BallotMeta{}, "not_found", nil
		}
		return BallotMeta{}, "", err
	}

	rows, err := s.db.Query(ctx, `SELECT id::text, name, meta FROM candidates WHERE election_id=$1::uuid ORDER BY name`, electionID)
	if err != nil {
		return BallotMeta{}, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var c Candidate
		var metaJSON []byte
		if err := rows.Scan(&c.ID, &c.Name, &metaJSON); err != nil {
			return BallotMeta{}, "", err
		}
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			_ = json.Unmarshal(metaJSON, &c.Meta)
		}
		meta.Candidates = append(meta.Candidates, c)
	}

	return meta, "", nil
}

func (s *Service) UpdateRules(ctx context.Context, electionID, adminUserID string, in UpdateRulesInput) (string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return "invalid_id", nil
	}

	// грузим текущее состояние (и проверяем ownership)
	var curStatus string
	var curTally, curFormat, curAccess string
	var curCommittee *int
	var curQuota *string
	var curPublishAt *time.Time
	var curShowAgg bool
	var curApproval *int
	var curTopK *int
	var curScoreMin *int
	var curScoreMax *int
	var curScoreStep *int
	var curScoreAllowSkip bool

	err := s.db.QueryRow(ctx, `
		SELECT status, tally_rule, ballot_format,
		       committee_size, quota_type,
		       access_mode, publish_at, show_aggregates,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip
		FROM elections
		WHERE id=$1::uuid AND created_by=$2::uuid
	`, electionID, adminUserID).Scan(
		&curStatus, &curTally, &curFormat,
		&curCommittee, &curQuota,
		&curAccess, &curPublishAt, &curShowAgg,
		&curApproval, &curTopK,
		&curScoreMin, &curScoreMax, &curScoreStep, &curScoreAllowSkip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "not_found", nil
		}
		return "", err
	}

	// Безопасно: правила меняем только пока выборы не стартовали
	if curStatus != "draft" && curStatus != "scheduled" {
		return "invalid_status", nil
	}

	// candidates count для проверки параметров (top-k, top-q)
	var candCount int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM candidates WHERE election_id=$1::uuid`, electionID).Scan(&candCount); err != nil {
		return "", err
	}

	// применяем изменения поверх текущих значений
	finalTally := curTally
	if in.TallyRule != nil {
		t, ok := validateTallyRule(*in.TallyRule)
		if !ok {
			return "invalid_tally_rule", nil
		}
		finalTally = t
	}

	finalFormat := curFormat
	if in.BallotFormat != nil {
		f := norm(*in.BallotFormat)
		if !allowedBallotFormats[f] {
			return "invalid_ballot_format", nil
		}
		finalFormat = f
	}

	finalCommittee := curCommittee
	if in.CommitteeSize != nil {
		if *in.CommitteeSize <= 0 {
			return "invalid_committee_size", nil
		}
		v := *in.CommitteeSize
		finalCommittee = &v
	}

	finalQuota := curQuota
	if in.QuotaType != nil {
		q := norm(*in.QuotaType)
		if q == "" || !allowedQuotaTypes[q] {
			return "invalid_quota_type", nil
		}
		finalQuota = &q
	}

	// committee/quota консистентность
	cs := 1
	if finalCommittee != nil {
		cs = *finalCommittee
	}
	if cs > 1 {
		if finalQuota == nil {
			return "quota_type_required", nil
		}
	} else {
		finalQuota = nil
	}

	finalAccess := curAccess
	if in.AccessMode != nil {
		a := norm(*in.AccessMode)
		if !allowedAccessModes[a] {
			return "invalid_access_mode", nil
		}
		finalAccess = a
	}

	finalPublishAt := curPublishAt
	if in.PublishAt != nil {
		p := strings.TrimSpace(*in.PublishAt)
		if p == "" {
			finalPublishAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, p)
			if err != nil {
				return "invalid_publish_at", nil
			}
			finalPublishAt = &t
		}
	}

	finalShowAgg := curShowAgg
	if in.ShowAggregates != nil {
		finalShowAgg = *in.ShowAggregates
	}

	finalApproval := curApproval
	if in.ApprovalMaxChoices != nil {
		v := *in.ApprovalMaxChoices
		finalApproval = &v
	}
	finalTopK := curTopK
	if in.RankingTopK != nil {
		v := *in.RankingTopK
		finalTopK = &v
	}
	finalScoreMin := curScoreMin
	if in.ScoreMin != nil {
		v := *in.ScoreMin
		finalScoreMin = &v
	}
	finalScoreMax := curScoreMax
	if in.ScoreMax != nil {
		v := *in.ScoreMax
		finalScoreMax = &v
	}
	finalScoreStep := curScoreStep
	if in.ScoreStep != nil {
		v := *in.ScoreStep
		finalScoreStep = &v
	}
	finalScoreAllowSkip := curScoreAllowSkip
	if in.ScoreAllowSkip != nil {
		finalScoreAllowSkip = *in.ScoreAllowSkip
	}

	// ballot params consistency
	if code := validateBallotParams(finalFormat, candCount, finalApproval, finalTopK, finalScoreMin, finalScoreMax, finalScoreStep); code != "" {
		return code, nil
	}

	_, err = s.db.Exec(ctx, `
		UPDATE elections SET
			tally_rule = $2,
			ballot_format = $3,
			committee_size = $4,
			quota_type = $5,
			access_mode = $6,
			publish_at = $7,
			show_aggregates = $8,
			approval_max_choices = $9,
			ranking_top_k = $10,
			score_min = $11,
			score_max = $12,
			score_step = $13,
			score_allow_skip = $14
		WHERE id=$1::uuid AND created_by=$15::uuid
	`, electionID,
		finalTally, finalFormat,
		finalCommittee, finalQuota,
		finalAccess,
		finalPublishAt,
		finalShowAgg,
		finalApproval, finalTopK,
		finalScoreMin, finalScoreMax, finalScoreStep,
		finalScoreAllowSkip,
		adminUserID,
	)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (s *Service) Action(ctx context.Context, electionID, adminUserID, action string) (string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return "invalid_id", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM elections
		WHERE id=$1::uuid AND created_by=$2::uuid
		FOR UPDATE
	`, electionID, adminUserID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "not_found", nil
		}
		return "", err
	}

	next, ok := nextStatus(status, action)
	if !ok {
		return "invalid_transition", nil
	}

	now := time.Now().UTC()

	switch action {
	case "close":
		_, err = tx.Exec(ctx, `UPDATE elections SET status=$2 WHERE id=$1::uuid`, electionID, next)
		if err != nil {
			return "", err
		}

		payload := map[string]any{
			"election_id": electionID,
			"action":      "close",
		}
		pb, _ := json.Marshal(payload)

		_, err = tx.Exec(ctx, `
			INSERT INTO jobs (kind, status, progress, created_by, election_id, payload)
			VALUES ('tally', 'queued', 0, $2::uuid, $1::uuid, $3::jsonb)
		`, electionID, adminUserID, string(pb))
		if err != nil {
			return "", err
		}

	case "publish":
		// 1) публикуем election
		_, err = tx.Exec(ctx, `UPDATE elections SET status=$2, published_at=$3 WHERE id=$1::uuid`, electionID, next, now)
		if err != nil {
			return "", err
		}

		// 2) проставляем published_at в последнем results
		tag, err := tx.Exec(ctx, `
			WITH latest AS (
				SELECT id
				FROM results
				WHERE election_id=$1::uuid
				ORDER BY version DESC
				LIMIT 1
			)
			UPDATE results r
			SET published_at=$2
			FROM latest
			WHERE r.id = latest.id
		`, electionID, now)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "no_results", nil
		}

	default:
		if action == "schedule" || action == "open" || action == "pause" || action == "resume" {
			_, err = tx.Exec(ctx, `UPDATE elections SET status=$2 WHERE id=$1::uuid`, electionID, next)
			if err != nil {
				return "", err
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE elections SET status=$2 WHERE id=$1::uuid`, electionID, next)
			if err != nil {
				return "", err
			}
		}
	}

	return "", tx.Commit(ctx)
}

func (s *Service) CreateInvite(ctx context.Context, electionID, adminUserID, email string) (InviteCreated, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return InviteCreated{}, "invalid_id", nil
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return InviteCreated{}, "invalid_email", nil
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return InviteCreated{}, "invalid_email", nil
	}

	var accessMode string
	err := s.db.QueryRow(ctx, `
		SELECT access_mode
		FROM elections
		WHERE id=$1::uuid AND created_by=$2::uuid
	`, electionID, adminUserID).Scan(&accessMode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InviteCreated{}, "not_found", nil
		}
		return InviteCreated{}, "", err
	}
	if accessMode != "invite" {
		return InviteCreated{}, "not_invite_mode", nil
	}

	code, codeHash := generateInviteCode()

	var inviteID string
	var createdAt time.Time
	err = s.db.QueryRow(ctx, `
		INSERT INTO election_invites (election_id, email, invite_code_hash, status)
		VALUES ($1::uuid, $2, $3, 'created')
		RETURNING id::text, created_at
	`, electionID, email, codeHash).Scan(&inviteID, &createdAt)
	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "unique") || strings.Contains(low, "duplicate") {
			return InviteCreated{}, "email_already_invited", nil
		}
		return InviteCreated{}, "", err
	}

	// audit (не критично, но полезно)
	_ = insertAudit(ctx, s.db, &adminUserID, "invite_created", map[string]any{
		"target_type": "election_invite",
		"target_id":   inviteID,
		"after": map[string]any{
			"election_id": electionID,
			"email":       email,
			"status":      "created",
		},
	})

	return InviteCreated{
		InviteID:   inviteID,
		Email:      email,
		InviteCode: code,
		Status:     "created",
		CreatedAt:  createdAt.UTC().Format(time.RFC3339),
	}, "", nil
}

func (s *Service) ListInvites(ctx context.Context, electionID, adminUserID string) ([]Invite, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return nil, "invalid_id", nil
	}

	var exists int
	err := s.db.QueryRow(ctx, `SELECT 1 FROM elections WHERE id=$1::uuid AND created_by=$2::uuid`, electionID, adminUserID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "not_found", nil
		}
		return nil, "", err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id::text, email, status, sent_at, accepted_at, created_at
		FROM election_invites
		WHERE election_id=$1::uuid
		ORDER BY created_at DESC
	`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []Invite
	for rows.Next() {
		var it Invite
		var sentAt, accAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&it.ID, &it.Email, &it.Status, &sentAt, &accAt, &createdAt); err != nil {
			return nil, "", err
		}
		it.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if sentAt != nil {
			sv := sentAt.UTC().Format(time.RFC3339)
			it.SentAt = &sv
		}
		if accAt != nil {
			sv := accAt.UTC().Format(time.RFC3339)
			it.AcceptedAt = &sv
		}
		out = append(out, it)
	}

	return out, "", nil
}

func (s *Service) isAccessible(ctx context.Context, electionID, userID, email, role string) (bool, error) {
	if role == "admin" {
		var x int
		err := s.db.QueryRow(ctx, `SELECT 1 FROM elections WHERE id=$1::uuid AND created_by=$2::uuid`, electionID, userID).Scan(&x)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	var x int
	err := s.db.QueryRow(ctx, `
		SELECT 1
		FROM elections e
		WHERE e.id=$1::uuid
		  AND (
			e.access_mode='open'
			OR EXISTS (
				SELECT 1 FROM election_invites i
				WHERE i.election_id=e.id
				  AND lower(i.email)=lower($2)
				  AND i.status IN ('created','sent','accepted')
			)
		  )
	`, electionID, email).Scan(&x)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func nextStatus(cur, action string) (string, bool) {
	switch action {
	case "schedule":
		return "scheduled", cur == "draft"
	case "open":
		return "active", cur == "scheduled"
	case "pause":
		return "paused", cur == "active"
	case "resume":
		return "active", cur == "paused"
	case "close":
		return "closed", cur == "active" || cur == "paused"
	case "publish":
		// сейчас допускаем publish из closed/results_ready (как у тебя было)
		return "published", cur == "closed" || cur == "results_ready"
	default:
		return "", false
	}
}

func generateInviteCode() (raw string, hashHex string) {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hashHex = hex.EncodeToString(h[:])
	return raw, hashHex
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// insertAudit: поддерживает tx и pool (оба имеют Exec). Это удобно, чтобы не плодить два варианта.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconnCommandTag, error)
}

type pgconnCommandTag interface {
	RowsAffected() int64
}

func insertAudit(ctx context.Context, tx any, actorUserID *string, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	// tx может быть pgx.Tx или *pgxpool.Pool — у обоих есть Exec, но типы разные.
	// Здесь делаем через небольшую унификацию:
	switch v := tx.(type) {
	case pgx.Tx:
		if actorUserID == nil {
			_, err = v.Exec(ctx,
				`INSERT INTO audit_log (actor_user_id, event_type, details)
				 VALUES (NULL, $1, $2::jsonb)`,
				eventType, string(b),
			)
			return err
		}
		_, err = v.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES ($1::uuid, $2, $3::jsonb)`,
			*actorUserID, eventType, string(b),
		)
		return err
	case *pgxpool.Pool:
		if actorUserID == nil {
			_, err = v.Exec(ctx,
				`INSERT INTO audit_log (actor_user_id, event_type, details)
				 VALUES (NULL, $1, $2::jsonb)`,
				eventType, string(b),
			)
			return err
		}
		_, err = v.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES ($1::uuid, $2, $3::jsonb)`,
			*actorUserID, eventType, string(b),
		)
		return err
	default:
		// неизвестный executor — молча не пишем, но и не валим основной флоу
		return nil
	}
}

type ElectionDetail struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`

	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`

	TallyRule    string `json:"tally_rule"`
	BallotFormat string `json:"ballot_format"`

	CommitteeSize *int    `json:"committee_size,omitempty"`
	QuotaType     *string `json:"quota_type,omitempty"`

	Status         string  `json:"status"`
	AccessMode     string  `json:"access_mode"`
	PublishAt      *string `json:"publish_at,omitempty"`
	PublishedAt    *string `json:"published_at,omitempty"`
	ShowAggregates bool    `json:"show_aggregates"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`
	ScoreMin           *int `json:"score_min,omitempty"`
	ScoreMax           *int `json:"score_max,omitempty"`
	ScoreStep          *int `json:"score_step,omitempty"`
	ScoreAllowSkip     bool `json:"score_allow_skip"`

	Candidates []Candidate `json:"candidates"`
}

func (s *Service) Get(ctx context.Context, electionID, userID, email, role string) (ElectionDetail, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return ElectionDetail{}, "invalid_id", nil
	}

	allowed, err := s.isAccessible(ctx, electionID, userID, email, role)
	if err != nil {
		return ElectionDetail{}, "", err
	}
	if !allowed {
		return ElectionDetail{}, "not_found", nil
	}

	var d ElectionDetail
	d.ID = electionID

	var startAt, endAt time.Time
	var publishAt, publishedAt *time.Time

	err = s.db.QueryRow(ctx, `
		SELECT title, description, start_at, end_at,
		       tally_rule, ballot_format,
		       committee_size, quota_type,
		       status, access_mode,
		       publish_at, published_at, show_aggregates,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip
		FROM elections
		WHERE id=$1::uuid
	`, electionID).Scan(
		&d.Title, &d.Description, &startAt, &endAt,
		&d.TallyRule, &d.BallotFormat,
		&d.CommitteeSize, &d.QuotaType,
		&d.Status, &d.AccessMode,
		&publishAt, &publishedAt, &d.ShowAggregates,
		&d.ApprovalMaxChoices, &d.RankingTopK,
		&d.ScoreMin, &d.ScoreMax, &d.ScoreStep, &d.ScoreAllowSkip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ElectionDetail{}, "not_found", nil
		}
		return ElectionDetail{}, "", err
	}

	d.StartAt = startAt.UTC().Format(time.RFC3339)
	d.EndAt = endAt.UTC().Format(time.RFC3339)
	if publishAt != nil {
		s := publishAt.UTC().Format(time.RFC3339)
		d.PublishAt = &s
	}
	if publishedAt != nil {
		s := publishedAt.UTC().Format(time.RFC3339)
		d.PublishedAt = &s
	}

	rows, err := s.db.Query(ctx, `SELECT id::text, name, meta FROM candidates WHERE election_id=$1::uuid ORDER BY name`, electionID)
	if err != nil {
		return ElectionDetail{}, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var c Candidate
		var metaJSON []byte
		if err := rows.Scan(&c.ID, &c.Name, &metaJSON); err != nil {
			return ElectionDetail{}, "", err
		}
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			_ = json.Unmarshal(metaJSON, &c.Meta)
		}
		d.Candidates = append(d.Candidates, c)
	}

	return d, "", nil
}

