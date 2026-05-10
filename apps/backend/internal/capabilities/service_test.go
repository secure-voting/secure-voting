package capabilities

import (
	"context"
	"net"
	"testing"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/computeclient"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fakeComputeServer struct {
	pb.UnimplementedComputeServer
	listTallyRulesFn func(ctx context.Context, in *emptypb.Empty) (*pb.ListTallyRulesResponse, error)
}

func (s *fakeComputeServer) ListTallyRules(ctx context.Context, in *emptypb.Empty) (*pb.ListTallyRulesResponse, error) {
	if s.listTallyRulesFn != nil {
		return s.listTallyRulesFn(ctx, in)
	}
	return &pb.ListTallyRulesResponse{}, nil
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestListTallyRules_WithoutCompute(t *testing.T) {
	svc := NewService(nil)

	items, err := svc.ListTallyRules(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "compute client unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %#v", items)
	}
}

func TestListTallyRules_Success(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	pb.RegisterComputeServer(srv, &fakeComputeServer{
		listTallyRulesFn: func(ctx context.Context, in *emptypb.Empty) (*pb.ListTallyRulesResponse, error) {
			return &pb.ListTallyRulesResponse{
				Rules: []*pb.TallyRuleInfo{
					{
						Id:                         "plurality",
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
						Id:                         "approval-2",
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
				},
			}, nil
		},
	})
	defer srv.Stop()

	go func() {
		_ = srv.Serve(lis)
	}()

	client, err := computeclient.New(context.Background(), computeclient.Config{
		Addr:   lis.Addr().String(),
		UseTLS: false,
	})
	if err != nil {
		t.Fatalf("computeclient.New returned error: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	svc := NewService(client)

	items, err := svc.ListTallyRules(context.Background())
	if err != nil {
		t.Fatalf("ListTallyRules returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != "plurality" {
		t.Fatalf("unexpected first id: %q", items[0].ID)
	}
	if len(items[0].BallotFormats) != 1 || items[0].BallotFormats[0] != "ranking" {
		t.Fatalf("unexpected first ballot formats: %#v", items[0].BallotFormats)
	}
	if !items[0].SupportsElectionTally {
		t.Fatal("expected plurality to support election tally")
	}

	if items[1].ID != "approval-2" {
		t.Fatalf("unexpected second id: %q", items[1].ID)
	}
	if !items[1].RequiresApprovalMaxChoices {
		t.Fatal("expected approval-2 to require approval_max_choices")
	}
}
