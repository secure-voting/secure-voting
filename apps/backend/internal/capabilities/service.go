package capabilities

import (
	"context"
	"errors"

	"secure-voting/apps/backend/internal/computeclient"
)

type Service struct {
	compute *computeclient.Client
}

func NewService(compute *computeclient.Client) *Service {
	return &Service{compute: compute}
}

func (s *Service) ListTallyRules(ctx context.Context) ([]computeclient.TallyRuleInfo, error) {
	if s.compute == nil {
		return nil, errors.New("compute client unavailable")
	}
	return s.compute.ListTallyRules(ctx)
}
