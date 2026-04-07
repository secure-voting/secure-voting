package capabilities

import (
	"context"

	"secure-voting/apps/backend/internal/computeclient"
)

type Service struct {
	compute *computeclient.Client
}

func NewService(compute *computeclient.Client) *Service {
	return &Service{compute: compute}
}

func (s *Service) ListTallyRules(ctx context.Context) ([]computeclient.TallyRuleInfo, error) {
	return s.compute.ListTallyRules(ctx)
}