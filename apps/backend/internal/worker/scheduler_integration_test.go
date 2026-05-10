package worker

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newSchedulerIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if os.Getenv("SECURE_VOTING_INTEGRATION") != "1" {
		t.Skip("set SECURE_VOTING_INTEGRATION=1 to run integration tests")
	}

	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_DSN must be set for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pg, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}

	t.Cleanup(func() {
		pg.Close()
	})

	return pg
}

func insertSchedulerTestUser(t *testing.T, pg *pgxpool.Pool) string {
	t.Helper()

	userID := uuid.NewString()
	email := "scheduler_" + strings.ToLower(strings.ReplaceAll(userID, "-", "")) + "@example.com"

	_, err := pg.Exec(context.Background(), `
		INSERT INTO users (id, email, password_hash, role)
		VALUES ($1::uuid, $2, $3, $4)
	`, userID, email, "integration-test-hash", "admin")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	return userID
}

func insertSchedulerCloseElection(t *testing.T, pg *pgxpool.Pool, createdBy string, status string) string {
	t.Helper()

	electionID := uuid.NewString()
	now := time.Now().UTC()

	_, err := pg.Exec(context.Background(), `
		INSERT INTO elections (
			id,
			title,
			description,
			start_at,
			end_at,
			tally_rule,
			ballot_format,
			committee_size,
			status,
			access_mode,
			show_aggregates,
			ranking_top_k,
			score_allow_skip,
			created_by
		) VALUES (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14::uuid
		)
	`, electionID,
		"scheduler close test",
		"integration",
		now.Add(-2*time.Hour),
		now.Add(-1*time.Minute),
		"borda",
		"ranking",
		1,
		status,
		"open",
		false,
		2,
		false,
		createdBy,
	)
	if err != nil {
		t.Fatalf("insert election: %v", err)
	}

	return electionID
}

func insertSchedulerPublishElection(t *testing.T, pg *pgxpool.Pool, createdBy string, withResult bool) string {
	t.Helper()

	electionID := uuid.NewString()
	now := time.Now().UTC()

	_, err := pg.Exec(context.Background(), `
		INSERT INTO elections (
			id,
			title,
			description,
			start_at,
			end_at,
			tally_rule,
			ballot_format,
			committee_size,
			status,
			access_mode,
			publish_at,
			show_aggregates,
			ranking_top_k,
			score_allow_skip,
			created_by
		) VALUES (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15::uuid
		)
	`, electionID,
		"scheduler publish test",
		"integration",
		now.Add(-3*time.Hour),
		now.Add(-2*time.Hour),
		"borda",
		"ranking",
		1,
		"results_ready",
		"open",
		now.Add(-1*time.Minute),
		false,
		2,
		false,
		createdBy,
	)
	if err != nil {
		t.Fatalf("insert election: %v", err)
	}

	if withResult {
		_, err = pg.Exec(context.Background(), `
			INSERT INTO results (
				id,
				election_id,
				version,
				method,
				params,
				winners,
				metrics,
				protocol,
				published_at
			) VALUES (
				$1::uuid,
				$2::uuid,
				1,
				'borda',
				'{"committee_size":1}'::jsonb,
				'["winner-1"]'::jsonb,
				'{"rounds":1}'::jsonb,
				'{"steps":["computed"]}'::jsonb,
				NULL
			)
		`, uuid.NewString(), electionID)
		if err != nil {
			t.Fatalf("insert result: %v", err)
		}
	}

	return electionID
}

func cleanupSchedulerArtifacts(t *testing.T, pg *pgxpool.Pool, electionID string, createdBy string) {
	t.Helper()

	_, _ = pg.Exec(context.Background(), `DELETE FROM jobs WHERE election_id = $1::uuid`, electionID)
	_, _ = pg.Exec(context.Background(), `DELETE FROM results WHERE election_id = $1::uuid`, electionID)
	_, _ = pg.Exec(context.Background(), `DELETE FROM audit_log WHERE details->>'target_id' = $1`, electionID)
	_, _ = pg.Exec(context.Background(), `DELETE FROM elections WHERE id = $1::uuid`, electionID)
	_, _ = pg.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, createdBy)
}

func TestIntegration_AutoCloseDueElections_ClosesAndQueuesTallyJob(t *testing.T) {
	pg := newSchedulerIntegrationDB(t)
	createdBy := insertSchedulerTestUser(t, pg)
	electionID := insertSchedulerCloseElection(t, pg, createdBy, "active")
	t.Cleanup(func() {
		cleanupSchedulerArtifacts(t, pg, electionID, createdBy)
	})

	w := &Worker{db: pg}

	if err := w.autoCloseDueElections(context.Background()); err != nil {
		t.Fatalf("autoCloseDueElections error: %v", err)
	}

	var status string
	err := pg.QueryRow(context.Background(), `
		SELECT status
		FROM elections
		WHERE id = $1::uuid
	`, electionID).Scan(&status)
	if err != nil {
		t.Fatalf("select election status: %v", err)
	}
	if status != "closed" {
		t.Fatalf("expected election status closed, got %q", status)
	}

	var tallyJobs int
	err = pg.QueryRow(context.Background(), `
		SELECT count(*)
		FROM jobs
		WHERE election_id = $1::uuid
		  AND kind = 'tally'
		  AND status = 'queued'
	`, electionID).Scan(&tallyJobs)
	if err != nil {
		t.Fatalf("count tally jobs: %v", err)
	}
	if tallyJobs != 1 {
		t.Fatalf("expected 1 queued tally job, got %d", tallyJobs)
	}

	var auditCount int
	err = pg.QueryRow(context.Background(), `
		SELECT count(*)
		FROM audit_log
		WHERE event_type = 'election_closed'
		  AND details->>'target_id' = $1
		  AND details->>'trigger' = 'scheduler'
	`, electionID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 scheduler close audit row, got %d", auditCount)
	}
}

func TestIntegration_AutoPublishDueElections_PublishesElectionAndLatestResult(t *testing.T) {
	pg := newSchedulerIntegrationDB(t)
	createdBy := insertSchedulerTestUser(t, pg)
	electionID := insertSchedulerPublishElection(t, pg, createdBy, true)
	t.Cleanup(func() {
		cleanupSchedulerArtifacts(t, pg, electionID, createdBy)
	})

	w := &Worker{db: pg}

	if err := w.autoPublishDueElections(context.Background()); err != nil {
		t.Fatalf("autoPublishDueElections error: %v", err)
	}

	var status string
	var publishedAt *time.Time
	err := pg.QueryRow(context.Background(), `
		SELECT status, published_at
		FROM elections
		WHERE id = $1::uuid
	`, electionID).Scan(&status, &publishedAt)
	if err != nil {
		t.Fatalf("select election status: %v", err)
	}
	if status != "published" {
		t.Fatalf("expected election status published, got %q", status)
	}
	if publishedAt == nil {
		t.Fatal("expected election published_at to be set")
	}

	var resultPublishedAt *time.Time
	err = pg.QueryRow(context.Background(), `
		SELECT published_at
		FROM results
		WHERE election_id = $1::uuid
		ORDER BY version DESC
		LIMIT 1
	`, electionID).Scan(&resultPublishedAt)
	if err != nil {
		t.Fatalf("select result published_at: %v", err)
	}
	if resultPublishedAt == nil {
		t.Fatal("expected latest result published_at to be set")
	}

	var auditCount int
	err = pg.QueryRow(context.Background(), `
		SELECT count(*)
		FROM audit_log
		WHERE event_type = 'election_published'
		  AND details->>'target_id' = $1
		  AND details->>'trigger' = 'scheduler'
	`, electionID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 scheduler publish audit row, got %d", auditCount)
	}
}

func TestIntegration_AutoPublishDueElections_SkipsWhenNoResults(t *testing.T) {
	pg := newSchedulerIntegrationDB(t)
	createdBy := insertSchedulerTestUser(t, pg)
	electionID := insertSchedulerPublishElection(t, pg, createdBy, false)
	t.Cleanup(func() {
		cleanupSchedulerArtifacts(t, pg, electionID, createdBy)
	})

	w := &Worker{db: pg}

	if err := w.autoPublishDueElections(context.Background()); err != nil {
		t.Fatalf("autoPublishDueElections error: %v", err)
	}

	var status string
	var publishedAt *time.Time
	err := pg.QueryRow(context.Background(), `
		SELECT status, published_at
		FROM elections
		WHERE id = $1::uuid
	`, electionID).Scan(&status, &publishedAt)
	if err != nil {
		t.Fatalf("select election status: %v", err)
	}
	if status != "results_ready" {
		t.Fatalf("expected election status to remain results_ready, got %q", status)
	}
	if publishedAt != nil {
		t.Fatal("expected election published_at to remain NULL")
	}

	var auditCount int
	err = pg.QueryRow(context.Background(), `
		SELECT count(*)
		FROM audit_log
		WHERE event_type = 'election_published'
		  AND details->>'target_id' = $1
		  AND details->>'trigger' = 'scheduler'
	`, electionID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("expected 0 scheduler publish audit rows, got %d", auditCount)
	}
}
