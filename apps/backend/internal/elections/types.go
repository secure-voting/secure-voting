package elections

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"secure-voting/apps/backend/internal/computeclient"
)

type Service struct {
	db           *pgxpool.Pool
	capabilities *computeclient.Client
}

func NewService(db *pgxpool.Pool, capabilities *computeclient.Client) *Service {
	return &Service{
		db:           db,
		capabilities: capabilities,
	}
}

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func normalizeRuleName(rule string) string {
	v := norm(rule)
	v = strings.ReplaceAll(v, "-", "_")

	switch v {
	case "minimax":
		return "minmax"
	case "condorcet_practical":
		return "practical_condorcet"
	default:
		return v
	}
}

func requiresCommitteeSize(rule string) bool {
	switch normalizeRuleName(rule) {
	case "plurality",
		"approval",
		"inverse_plurality",
		"borda",
		"black",
		"copeland_i",
		"copeland_ii",
		"copeland_iii",
		"simpson",
		"minmax",
		"hare",
		"inverse_borda",
		"nanson",
		"coombs",
		"practical_condorcet",
		"threshold":
		return true
	default:
		return false
	}
}

func normalizeCommitteeSize(rule string, committeeSize *int, candidateCount int) (*int, error) {
	if candidateCount < 2 {
		return nil, errors.New("at least 2 candidates required")
	}

	if !requiresCommitteeSize(rule) {
		return nil, nil
	}

	if committeeSize == nil {
		return nil, errors.New("committee_size is required for selected tally rule")
	}

	if *committeeSize < 1 {
		return nil, errors.New("committee_size must be >= 1")
	}

	if *committeeSize > candidateCount {
		return nil, fmt.Errorf("committee_size must be <= candidates count (%d)", candidateCount)
	}

	v := *committeeSize
	return &v, nil
}

func normalizeRankingTopK(ballotFormat string, rankingTopK *int, candidateCount int) (*int, error) {
	format := norm(ballotFormat)

	if format != "ranking" {
		return nil, nil
	}

	if rankingTopK == nil {
		return nil, nil
	}

	if *rankingTopK < 1 {
		return nil, errors.New("ranking_top_k must be >= 1")
	}

	if candidateCount < 1 {
		return nil, errors.New("candidate count must be >= 1")
	}

	v := *rankingTopK
	if v > candidateCount {
		v = candidateCount
	}

	return &v, nil
}

func normalizeCandidateName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

func normalizeCandidateDescription(desc string) string {
	return strings.TrimSpace(desc)
}

func normalizeCandidatesFromObjects(items []CandidateInput) ([]CandidateNormalized, error) {
	result := make([]CandidateNormalized, 0, len(items))
	seen := make(map[string]struct{}, len(items))

	for i, item := range items {
		name := normalizeCandidateName(item.Name)
		if name == "" {
			return nil, fmt.Errorf("candidate #%d: empty name", i+1)
		}
		if len(name) < 2 {
			return nil, fmt.Errorf("candidate #%d: name too short", i+1)
		}
		if len(name) > 200 {
			return nil, fmt.Errorf("candidate #%d: name too long", i+1)
		}

		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate candidate name: %s", name)
		}
		seen[key] = struct{}{}

		meta := map[string]any{}
		for k, v := range item.Meta {
			meta[k] = v
		}

		if item.Description != nil {
			desc := normalizeCandidateDescription(*item.Description)
			if len(desc) > 1000 {
				return nil, fmt.Errorf("candidate %s: description too long", name)
			}
			if desc == "" {
				delete(meta, "description")
			} else {
				meta["description"] = desc
			}
		} else if rawDesc, ok := meta["description"].(string); ok {
			desc := normalizeCandidateDescription(rawDesc)
			if len(desc) > 1000 {
				return nil, fmt.Errorf("candidate %s: description too long", name)
			}
			if desc == "" {
				delete(meta, "description")
			} else {
				meta["description"] = desc
			}
		}

		result = append(result, CandidateNormalized{
			Name: name,
			Meta: meta,
		})
	}

	if len(result) < 2 {
		return nil, errors.New("at least 2 candidates required")
	}

	return result, nil
}

func normalizeCandidatesFromNames(items []string) ([]CandidateNormalized, error) {
	objects := make([]CandidateInput, 0, len(items))
	for _, name := range items {
		objects = append(objects, CandidateInput{
			Name: name,
		})
	}
	return normalizeCandidatesFromObjects(objects)
}

func extractNormalizedCandidates(candidates []CandidateInput, candidateNames []string) ([]CandidateNormalized, error) {
	if len(candidates) > 0 {
		return normalizeCandidatesFromObjects(candidates)
	}
	return normalizeCandidatesFromNames(candidateNames)
}

func candidateNormalizationCode(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	switch {
	case msg == "at least 2 candidates required":
		return "candidates_required"
	case strings.Contains(msg, "empty name"),
		strings.Contains(msg, "name too short"),
		strings.Contains(msg, "name too long"):
		return "invalid_candidate_name"
	case strings.Contains(msg, "duplicate candidate name"):
		return "duplicate_candidate_name"
	case strings.Contains(msg, "description too long"):
		return "invalid_candidate_description"
	default:
		return "invalid_candidates"
	}
}

func committeeSizeCode(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	switch {
	case msg == "at least 2 candidates required":
		return "candidates_required"
	case msg == "committee_size is required for selected tally rule":
		return "committee_size_required"
	case msg == "committee_size must be >= 1":
		return "invalid_committee_size"
	case strings.Contains(msg, "committee_size must be <="):
		return "committee_size_too_large"
	default:
		return "invalid_committee_size"
	}
}

func rankingTopKCode(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	switch {
	case msg == "candidate count must be >= 1":
		return "candidates_required"
	case msg == "ranking_top_k must be >= 1":
		return "invalid_ranking_top_k"
	default:
		return "invalid_ranking_top_k"
	}
}

func canOpenElection(status string) bool {
	switch norm(status) {
	case "draft", "scheduled":
		return true
	default:
		return false
	}
}

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
	n = strings.ReplaceAll(n, "-", "_")
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

func validateBallotParams(
	format string,
	candidatesCount int,
	approvalMaxChoices *int,
	rankingTopK *int,
	scoreMin *int,
	scoreMax *int,
	scoreStep *int,
) string {
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
			return ""
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

var (
	ErrInvalidTallyRule           = errors.New("invalid_tally_rule")
	ErrIncompatibleBallotFormat   = errors.New("incompatible_ballot_format")
	ErrInvalidCommitteeSize       = errors.New("invalid_committee_size")
	ErrUnsupportedQuota           = errors.New("unsupported_quota")
	ErrMissingApprovalMaxChoices  = errors.New("missing_approval_max_choices")
	ErrUnsupportedTopK            = errors.New("unsupported_top_k")
	ErrInvalidScoreRange          = errors.New("invalid_score_range")
)

func validateRuleCompatibility(
	rule string,
	format string,
	params map[string]any,
	rules []computeclient.TallyRuleInfo,
) error {
	matrix := buildRuleMatrix(rules)

	info, ok := matrix.get(rule)
	if !ok {
		return ErrInvalidTallyRule
	}

	// ballot format
	allowed := false
	for _, f := range info.BallotFormats {
		if f == format {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrIncompatibleBallotFormat
	}

	// committee size
	if info.RequiresCommitteeSize {
		if v, ok := params["committee_size"].(int); !ok || v < 1 {
			return ErrInvalidCommitteeSize
		}
	}

	// quota
	if !info.SupportsQuotaType {
		if params["quota_type"] != nil {
			return ErrUnsupportedQuota
		}
	}

	// approval
	if info.RequiresApprovalMaxChoices {
		if params["approval_max_choices"] == nil {
			return ErrMissingApprovalMaxChoices
		}
	}

	// ranking
	if !info.SupportsRankingTopK {
		if params["ranking_top_k"] != nil {
			return ErrUnsupportedTopK
		}
	}

	// score
	if info.RequiresScoreRange {
		if params["score_min"] == nil ||
			params["score_max"] == nil ||
			params["score_step"] == nil {
			return ErrInvalidScoreRange
		}
	}

	return nil
}

type CandidateInput struct {
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

type CandidateNormalized struct {
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

	Candidates     []CandidateInput `json:"candidates"`
	CandidateNames []string         `json:"candidate_names,omitempty"`
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
	InviteID              string `json:"invite_id"`
	Email                 string `json:"email"`
	InviteCode            string `json:"invite_code,omitempty"`
	Status                string `json:"status"`
	CreatedAt             string `json:"created_at"`
	RegistrationRequired  bool   `json:"registration_required"`
	RegistrationEmailSent bool   `json:"registration_email_sent"`
	InviteEmailSent       bool   `json:"invite_email_sent"`
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
