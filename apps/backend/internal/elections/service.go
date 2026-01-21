package elections

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	ScoreMin        *int  `json:"score_min,omitempty"`
	ScoreMax        *int  `json:"score_max,omitempty"`
	ScoreStep       *int  `json:"score_step,omitempty"`
	ScoreAllowSkip  bool  `json:"score_allow_skip"`
	Candidates      []CandidateInput `json:"candidates"`
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
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Status    string  `json:"status"`
	SentAt    *string `json:"sent_at,omitempty"`
	AcceptedAt *string `json:"accepted_at,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type InviteCreated struct {
	InviteID   string `json:"invite_id"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"` // показываем 1 раз
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

func (s *Service) Create(ctx context.Context, createdBy string, in CreateElectionInput) (string, string, error) {
	if strings.TrimSpace(in.Title) == "" {
		return "", "invalid_title", nil
	}
	startAt, err := time.Parse(time.RFC3339, in.StartAt)
	if err != nil {
		return "", "invalid_start_at", nil
	}
	endAt, err := time.Parse(time.RFC3339, in.EndAt)
	if err != nil {
		return "", "invalid_end_at", nil
	}
	if !startAt.Before(endAt) {
		return "", "invalid_time_range", nil
	}

	switch in.BallotFormat {
	case "approval", "ranking", "score":
	default:
		return "", "invalid_ballot_format", nil
	}
	switch in.AccessMode {
	case "open", "invite":
	default:
		return "", "invalid_access_mode", nil
	}

	var publishAt *time.Time
	if in.PublishAt != nil && strings.TrimSpace(*in.PublishAt) != "" {
		t, err := time.Parse(time.RFC3339, *in.PublishAt)
		if err != nil {
			return "", "invalid_publish_at", nil
		}
		publishAt = &t
	}

	// кандидаты
	if len(in.Candidates) == 0 {
		return "", "candidates_required", nil
	}
	for _, c := range in.Candidates {
		if strings.TrimSpace(c.Name) == "" {
			return "", "invalid_candidate_name", nil
		}
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
	`, in.Title, in.Description, startAt, endAt, in.TallyRule, in.BallotFormat,
		in.CommitteeSize, in.QuotaType,
		in.AccessMode,
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
			s := publishedAt.UTC().Format(time.RFC3339)
			e.PublishedAt = &s
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

	// only own election
	var exists int
	err := s.db.QueryRow(ctx, `SELECT 1 FROM elections WHERE id=$1::uuid AND created_by=$2::uuid`, electionID, adminUserID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "not_found", nil
		}
		return "", err
	}

	var publishAt *time.Time
	if in.PublishAt != nil && strings.TrimSpace(*in.PublishAt) != "" {
		t, err := time.Parse(time.RFC3339, *in.PublishAt)
		if err != nil {
			return "invalid_publish_at", nil
		}
		publishAt = &t
	}

	_, err = s.db.Exec(ctx, `
		UPDATE elections SET
			tally_rule = COALESCE($2, tally_rule),
			ballot_format = COALESCE($3, ballot_format),
			committee_size = COALESCE($4, committee_size),
			quota_type = COALESCE($5, quota_type),
			access_mode = COALESCE($6, access_mode),
			publish_at = COALESCE($7, publish_at),
			show_aggregates = COALESCE($8, show_aggregates),
			approval_max_choices = COALESCE($9, approval_max_choices),
			ranking_top_k = COALESCE($10, ranking_top_k),
			score_min = COALESCE($11, score_min),
			score_max = COALESCE($12, score_max),
			score_step = COALESCE($13, score_step),
			score_allow_skip = COALESCE($14, score_allow_skip)
		WHERE id=$1::uuid
	`, electionID,
		in.TallyRule, in.BallotFormat,
		in.CommitteeSize, in.QuotaType,
		in.AccessMode,
		publishAt,
		in.ShowAggregates,
		in.ApprovalMaxChoices, in.RankingTopK,
		in.ScoreMin, in.ScoreMax, in.ScoreStep,
		in.ScoreAllowSkip,
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

	var status string
	err := s.db.QueryRow(ctx, `SELECT status FROM elections WHERE id=$1::uuid AND created_by=$2::uuid`, electionID, adminUserID).Scan(&status)
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

	if action == "publish" {
		_, err = s.db.Exec(ctx, `UPDATE elections SET status=$2, published_at=now() WHERE id=$1::uuid`, electionID, next)
	} else {
		_, err = s.db.Exec(ctx, `UPDATE elections SET status=$2 WHERE id=$1::uuid`, electionID, next)
	}
	if err != nil {
		return "", err
	}
	return "", nil
}

func (s *Service) CreateInvite(ctx context.Context, electionID, adminUserID, email string) (InviteCreated, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return InviteCreated{}, "invalid_id", nil
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
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
			s := sentAt.UTC().Format(time.RFC3339)
			it.SentAt = &s
		}
		if accAt != nil {
			s := accAt.UTC().Format(time.RFC3339)
			it.AcceptedAt = &s
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
		return "published", cur == "closed" || cur == "results_ready"
	default:
		return "", false
	}
}

func generateInviteCode() (raw string, hashHex string) {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b) // 24 chars
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

func insertAudit(ctx context.Context, tx pgx.Tx, actorUserID *string, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if actorUserID == nil {
		_, err = tx.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES (NULL, $1, $2::jsonb)`,
			eventType, string(b),
		)
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_user_id, event_type, details)
		 VALUES ($1::uuid, $2, $3::jsonb)`,
		*actorUserID, eventType, string(b),
	)
	return err
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

	Status      string `json:"status"`
	AccessMode  string `json:"access_mode"`
	PublishAt   *string `json:"publish_at,omitempty"`
	PublishedAt *string `json:"published_at,omitempty"`
	ShowAggregates bool `json:"show_aggregates"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`
	ScoreMin           *int `json:"score_min,omitempty"`
	ScoreMax           *int `json:"score_max,omitempty"`
	ScoreStep          *int `json:"score_step,omitempty"`
	ScoreAllowSkip     bool `json:"score_allow_skip"`

	Candidates []Candidate `json:"candidates"`
}

func (s *Service) Get(ctx context.Context, electionID, userID, email, role string) (ElectionDetail, string, error) {
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
