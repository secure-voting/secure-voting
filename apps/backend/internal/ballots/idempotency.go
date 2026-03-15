package ballots

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *Service) tryGetCached(ctx context.Context, rkey string) (SubmitResp, bool) {
	if s.rdb == nil {
		return SubmitResp{}, false
	}
	val, err := s.rdb.Get(ctx, rkey).Result()
	if err != nil {
		return SubmitResp{}, false
	}
	if strings.TrimSpace(val) == "" {
		return SubmitResp{}, false
	}
	var cached SubmitResp
	if json.Unmarshal([]byte(val), &cached) != nil {
		return SubmitResp{}, false
	}
	if cached.BallotID == "" || cached.Status == "" {
		return SubmitResp{}, false
	}
	return cached, true
}

func (s *Service) cacheResp(ctx context.Context, rkey string, resp SubmitResp) {
	if s.rdb == nil {
		return
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, rkey, string(b), s.idemTTL).Err()
}
