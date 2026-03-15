package experimentruns

import (
	"context"
	"encoding/json"
)

func (s *Service) DownloadResult(ctx context.Context, role, userID, runID string) ([]byte, string, string, string, error) {
	res, code, err := s.GetResult(ctx, role, userID, runID)
	if err != nil {
		return nil, "", "", "", err
	}
	if code != "" {
		return nil, "", "", code, nil
	}

	b, _ := json.Marshal(res)
	return b, "experiment_result_" + runID + ".json", "application/json", "", nil
}
