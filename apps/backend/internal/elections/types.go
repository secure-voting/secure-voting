package elections

import (
	"strings"

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

var allowedTallyRules = map[string]bool{
	"plurality":           true,
	"approval":            true,
	"inverse_plurality":   true,
	"borda":               true,
	"black":               true,
	"copeland_i":          true,
	"copeland_ii":         true,
	"copeland_iii":        true,
	"simpson":             true,
	"minmax":              true,
	"hare":                true,
	"inverse_borda":       true,
	"nanson":              true,
	"coombs":              true,
	"practical_condorcet": true,
	"threshold":           true,
}

var tallyRuleAliases = map[string]string{
	"minimax":             "minmax",
	"condorcet_practical": "practical_condorcet",
}

func validateTallyRule(v string) (string, bool) {
	n := norm(v)
	if n == "" {
		return "", false
	}
	if alias, ok := tallyRuleAliases[n]; ok {
		n = alias
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
