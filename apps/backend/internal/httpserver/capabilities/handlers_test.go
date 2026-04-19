package capabilities

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-voting/apps/backend/internal/computeclient"
	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type apiErrResp struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErr(t *testing.T, rr *httptest.ResponseRecorder) apiErrResp {
	t.Helper()

	var er apiErrResp
	if err := json.Unmarshal(rr.Body.Bytes(), &er); err != nil {
		t.Fatalf("failed to decode error response: %v; body=%s", err, rr.Body.String())
	}
	return er
}

func TestListTallyRules_Success_ReturnsItems(t *testing.T) {
	h := &Handlers{
		listTallyRules: func(ctx context.Context) ([]computeclient.TallyRuleInfo, error) {
			return []computeclient.TallyRuleInfo{
				{
					ID:                         "plurality",
					Label:                      "Plurality",
					BallotFormats:              []string{"ranking"},
					SupportsElectionTally:      true,
					SupportsExperimentRuns:     true,
					RequiresCommitteeSize:      true,
					SupportsQuotaType:          false,
					RequiresApprovalMaxChoices: false,
					SupportsRankingTopK:        true,
					RequiresScoreRange:         false,
				},
				{
					ID:                         "approval-2",
					Label:                      "Approval-2",
					BallotFormats:              []string{"approval"},
					SupportsElectionTally:      false,
					SupportsExperimentRuns:     true,
					RequiresCommitteeSize:      true,
					SupportsQuotaType:          false,
					RequiresApprovalMaxChoices: true,
					SupportsRankingTopK:        false,
					RequiresScoreRange:         false,
				},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/capabilities/tally-rules", nil)
	rr := httptest.NewRecorder()

	httputil.Wrap(h.ListTallyRules).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items []computeclient.TallyRuleInfo `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != "plurality" {
		t.Fatalf("unexpected first id: %q", resp.Items[0].ID)
	}
	if len(resp.Items[0].BallotFormats) != 1 || resp.Items[0].BallotFormats[0] != "ranking" {
		t.Fatalf("unexpected first ballot formats: %+v", resp.Items[0].BallotFormats)
	}

	if resp.Items[1].ID != "approval-2" {
		t.Fatalf("unexpected second id: %q", resp.Items[1].ID)
	}
	if len(resp.Items[1].BallotFormats) != 1 || resp.Items[1].BallotFormats[0] != "approval" {
		t.Fatalf("unexpected second ballot formats: %+v", resp.Items[1].BallotFormats)
	}
	if !resp.Items[1].RequiresApprovalMaxChoices {
		t.Fatalf("expected approval rule to require approval_max_choices")
	}
}

func TestListTallyRules_InternalError_MapsTo500(t *testing.T) {
	h := &Handlers{
		listTallyRules: func(ctx context.Context) ([]computeclient.TallyRuleInfo, error) {
			return nil, errors.New("boom")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/capabilities/tally-rules", nil)
	rr := httptest.NewRecorder()

	httputil.Wrap(h.ListTallyRules).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "internal_error" {
		t.Fatalf("unexpected error code: %+v", er)
	}
	if er.Error.Message != "list tally rules failed" {
		t.Fatalf("unexpected error message: %+v", er)
	}
}