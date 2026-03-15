package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/db"
)

type integrationEnv struct {
	cfg    config.Config
	pg     *pgxpool.Pool
	rdb    *redis.Client
	mc     *mongo.Client
	mdb    *mongo.Database
	server *httptest.Server
	client *http.Client
}

func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()

	if os.Getenv("SECURE_VOTING_INTEGRATION") != "1" {
		t.Skip("set SECURE_VOTING_INTEGRATION=1 to run integration tests")
	}

	pgDSN := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	mongoURI := strings.TrimSpace(os.Getenv("MONGO_URI"))
	mongoDB := strings.TrimSpace(os.Getenv("MONGO_DB"))
	redisPassword := os.Getenv("REDIS_PASSWORD")

	if pgDSN == "" || redisAddr == "" || mongoURI == "" {
		t.Skip("POSTGRES_DSN, REDIS_ADDR and MONGO_URI must be set for integration tests")
	}
	if mongoDB == "" {
		mongoDB = "secure_voting"
	}

	cfg := config.FromEnv()
	cfg.PostgresDSN = pgDSN
	cfg.RedisAddr = redisAddr
	cfg.RedisPassword = redisPassword
	cfg.MongoURI = mongoURI
	cfg.MongoDBName = mongoDB

	bootCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pg, err := db.NewPostgresPool(bootCtx, cfg.PostgresDSN)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}

	rdb, err := db.NewRedisClient(bootCtx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		pg.Close()
		t.Fatalf("redis: %v", err)
	}

	mc, err := db.NewMongoClient(bootCtx, cfg.MongoURI)
	if err != nil {
		_ = rdb.Close()
		pg.Close()
		t.Fatalf("mongo: %v", err)
	}
	mdb := mc.Database(cfg.MongoDBName)

	handler := Routes(cfg, pg, rdb, mdb)
	srv := httptest.NewServer(handler)

	env := &integrationEnv{
		cfg:    cfg,
		pg:     pg,
		rdb:    rdb,
		mc:     mc,
		mdb:    mdb,
		server: srv,
		client: srv.Client(),
	}

	t.Cleanup(func() {
		srv.Close()
		_ = rdb.Close()
		_ = mc.Disconnect(context.Background())
		pg.Close()
	})

	return env
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s_%d@example.com", prefix, time.Now().UnixNano())
}

func doJSONRequest(t *testing.T, client *http.Client, method, url, token string, body any) (int, map[string]any, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if len(raw) == 0 {
		return resp.StatusCode, nil, raw
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal response (%d): %v; body=%s", resp.StatusCode, err, string(raw))
	}

	return resp.StatusCode, data, raw
}

func doJSONRequestWithHeaders(t *testing.T, client *http.Client, method, url, token string, body any, headers map[string]string) (int, map[string]any, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if len(raw) == 0 {
		return resp.StatusCode, nil, raw
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal response (%d): %v; body=%s", resp.StatusCode, err, string(raw))
	}

	return resp.StatusCode, data, raw
}

func mustGetString(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in response: %#v", key, m)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("key %q is not string: %#v", key, v)
	}
	return s
}

func upsertUser(t *testing.T, pg *pgxpool.Pool, email, password, role string) {
	t.Helper()

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	_, err = pg.Exec(context.Background(), `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (email)
		DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role
	`, strings.ToLower(strings.TrimSpace(email)), string(hashBytes), role)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
}

func getUserRole(t *testing.T, pg *pgxpool.Pool, email string) string {
	t.Helper()

	var role string
	err := pg.QueryRow(context.Background(),
		`SELECT role FROM users WHERE lower(email)=lower($1)`,
		email,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ""
		}
		t.Fatalf("get user role: %v", err)
	}
	return role
}

func loginAndGetToken(t *testing.T, env *integrationEnv, email, password string) string {
	t.Helper()

	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	if status != http.StatusOK {
		t.Fatalf("login status=%d body=%s", status, string(raw))
	}

	return mustGetString(t, data, "access_token")
}

func createElectionAsAdmin(t *testing.T, env *integrationEnv, token, accessMode string) string {
	t.Helper()

	now := time.Now().UTC()

	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/elections", token, map[string]any{
		"title":         "Integration election",
		"description":   "integration-test",
		"start_at":      now.Add(1 * time.Hour).Format(time.RFC3339),
		"end_at":        now.Add(2 * time.Hour).Format(time.RFC3339),
		"tally_rule":    "borda",
		"ballot_format": "ranking",
		"committee_size": 1,
		"ranking_top_k":  2,
		"access_mode":    accessMode,
		"candidates": []map[string]any{
			{"name": "Alice"},
			{"name": "Bob"},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("create election status=%d body=%s", status, string(raw))
	}

	return mustGetString(t, data, "id")
}

func responseErrorCode(t *testing.T, data map[string]any) string {
	t.Helper()

	if data == nil {
		t.Fatal("response body is nil")
	}

	errV, ok := data["error"]
	if !ok {
		t.Fatalf("missing error object in response: %#v", data)
	}

	errObj, ok := errV.(map[string]any)
	if !ok {
		t.Fatalf("error is not an object: %#v", errV)
	}

	code, ok := errObj["code"].(string)
	if !ok || code == "" {
		t.Fatalf("missing error.code in response: %#v", errObj)
	}

	return code
}

func mustAction(t *testing.T, env *integrationEnv, token, electionID, action string) {
	t.Helper()

	status, _, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/elections/"+electionID+"/actions/"+action,
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("action %s status=%d body=%s", action, status, string(raw))
	}
}

func listContainsElectionID(t *testing.T, data map[string]any, electionID string) bool {
	t.Helper()

	itemsV, ok := data["items"]
	if !ok {
		t.Fatalf("missing items: %#v", data)
	}

	items, ok := itemsV.([]any)
	if !ok {
		t.Fatalf("items is not array: %#v", itemsV)
	}

	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id, _ := obj["id"].(string)
		if id == electionID {
			return true
		}
	}
	return false
}

func listContainsExperimentID(t *testing.T, data map[string]any, experimentID string) bool {
	t.Helper()

	itemsV, ok := data["items"]
	if !ok {
		t.Fatalf("missing items: %#v", data)
	}

	items, ok := itemsV.([]any)
	if !ok {
		t.Fatalf("items is not array: %#v", itemsV)
	}

	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id, _ := obj["id"].(string)
		if id == experimentID {
			return true
		}
	}
	return false
}

func candidateIDsFromBallotMeta(t *testing.T, data map[string]any) []string {
	t.Helper()

	itemsV, ok := data["candidates"]
	if !ok {
		t.Fatalf("missing candidates: %#v", data)
	}

	items, ok := itemsV.([]any)
	if !ok {
		t.Fatalf("candidates is not array: %#v", itemsV)
	}

	out := make([]string, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id, _ := obj["id"].(string)
		if id != "" {
			out = append(out, id)
		}
	}

	if len(out) == 0 {
		t.Fatalf("no candidate ids extracted from ballot meta: %#v", data)
	}

	return out
}

func insertElectionResult(t *testing.T, pg *pgxpool.Pool, electionID string) {
	t.Helper()

	_, err := pg.Exec(context.Background(), `
		INSERT INTO results (
			election_id,
			version,
			method,
			params,
			winners,
			metrics,
			protocol,
			published_at
		)
		VALUES (
			$1::uuid,
			1,
			'borda',
			'{"committee_size":1}'::jsonb,
			'["winner-1"]'::jsonb,
			'{"rounds":1}'::jsonb,
			'{"steps":["computed"]}'::jsonb,
			NULL
		)
	`, electionID)
	if err != nil {
		t.Fatalf("insert result: %v", err)
	}
}

func TestIntegration_RegisterDoesNotGrantPrivilegedRole(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("register_role")
	password := "StrongPass123!"

	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/auth/register", "", map[string]any{
		"email":    email,
		"password": password,
		"role":     "admin",
	})

	switch status {
	case http.StatusOK:
		userV, ok := data["user"]
		if !ok {
			t.Fatalf("missing user in response: %s", string(raw))
		}
		user, ok := userV.(map[string]any)
		if !ok {
			t.Fatalf("user is not object: %#v", userV)
		}
		role, _ := user["role"].(string)
		if role != "voter" {
			t.Fatalf("expected returned role voter, got %q; body=%s", role, string(raw))
		}

	case http.StatusBadRequest:
		// Тоже безопасный вариант: API мог начать отвергать поле role.
	default:
		t.Fatalf("unexpected status=%d body=%s", status, string(raw))
	}

	dbRole := getUserRole(t, env.pg, email)
	if dbRole == "admin" || dbRole == "researcher" {
		t.Fatalf("privileged role was granted in db: %q", dbRole)
	}
}

func TestIntegration_InviteOnlyElectionRequiresAcceptedInvite(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_invite")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	voterEmail := uniqueEmail("voter_invite")
	voterPassword := "StrongPass123!"
	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/auth/register", "", map[string]any{
		"email":    voterEmail,
		"password": voterPassword,
	})
	if status != http.StatusOK {
		t.Fatalf("register voter status=%d body=%s", status, string(raw))
	}
	voterToken := mustGetString(t, data, "access_token")

	electionID := createElectionAsAdmin(t, env, adminToken, "invite")
	mustAction(t, env, adminToken, electionID, "schedule")
	mustAction(t, env, adminToken, electionID, "open")

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/elections/"+electionID+"/invites",
		adminToken,
		map[string]any{"email": voterEmail},
	)
	if status != http.StatusOK {
		t.Fatalf("create invite status=%d body=%s", status, string(raw))
	}

	status, listResp, raw := doJSONRequest(t, env.client, http.MethodGet, env.server.URL+"/api/v1/elections", voterToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list elections status=%d body=%s", status, string(raw))
	}
	if listContainsElectionID(t, listResp, electionID) {
		t.Fatalf("invite-only election is visible before accepted invite")
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/elections/"+electionID+"/ballot",
		voterToken,
		nil,
	)
	if status != http.StatusNotFound {
		t.Fatalf("ballot meta should be not found before accepted invite; status=%d body=%s", status, string(raw))
	}

	_, err := env.pg.Exec(context.Background(), `
		UPDATE election_invites
		SET status='accepted', accepted_at=now()
		WHERE election_id=$1::uuid AND lower(email)=lower($2)
	`, electionID, voterEmail)
	if err != nil {
		t.Fatalf("accept invite in db: %v", err)
	}

	status, listResp, raw = doJSONRequest(t, env.client, http.MethodGet, env.server.URL+"/api/v1/elections", voterToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list elections after accept status=%d body=%s", status, string(raw))
	}
	if !listContainsElectionID(t, listResp, electionID) {
		t.Fatalf("invite-only election is not visible after accepted invite")
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/elections/"+electionID+"/ballot",
		voterToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("ballot meta should be accessible after accepted invite; status=%d body=%s", status, string(raw))
	}
}

func TestIntegration_ExperimentsACLByOwner(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_exp")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	r1Email := uniqueEmail("researcher1")
	r1Password := "StrongPass123!"
	upsertUser(t, env.pg, r1Email, r1Password, "researcher")
	r1Token := loginAndGetToken(t, env, r1Email, r1Password)

	r2Email := uniqueEmail("researcher2")
	r2Password := "StrongPass123!"
	upsertUser(t, env.pg, r2Email, r2Password, "researcher")
	r2Token := loginAndGetToken(t, env, r2Email, r2Password)

	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/experiments", r1Token, map[string]any{
		"type": "algo",
		"params": map[string]any{
			"ballot_format":  "ranking",
			"tally_rule":     "borda",
			"committee_size": 1,
		},
	})
	if status != http.StatusOK {
		t.Fatalf("create experiment status=%d body=%s", status, string(raw))
	}
	experimentID := mustGetString(t, data, "id")

	status, _, raw = doJSONRequest(t, env.client, http.MethodGet, env.server.URL+"/api/v1/experiments/"+experimentID, r2Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("researcher2 should not access researcher1 experiment; status=%d body=%s", status, string(raw))
	}

	status, data, raw = doJSONRequest(t, env.client, http.MethodGet, env.server.URL+"/api/v1/experiments", r2Token, nil)
	if status != http.StatusOK {
		t.Fatalf("researcher2 list status=%d body=%s", status, string(raw))
	}
	if listContainsExperimentID(t, data, experimentID) {
		t.Fatalf("researcher2 list unexpectedly contains researcher1 experiment")
	}

	status, _, raw = doJSONRequest(t, env.client, http.MethodGet, env.server.URL+"/api/v1/experiments/"+experimentID, adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("admin should access experiment; status=%d body=%s", status, string(raw))
	}
}

func TestIntegration_BallotSubmitIdempotencyAndAlreadySubmitted(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_vote")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	voterEmail := uniqueEmail("voter_vote")
	voterPassword := "StrongPass123!"
	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/auth/register", "", map[string]any{
		"email":    voterEmail,
		"password": voterPassword,
	})
	if status != http.StatusOK {
		t.Fatalf("register voter status=%d body=%s", status, string(raw))
	}
	voterToken := mustGetString(t, data, "access_token")

	electionID := createElectionAsAdmin(t, env, adminToken, "open")
	mustAction(t, env, adminToken, electionID, "schedule")
	mustAction(t, env, adminToken, electionID, "open")

	status, ballotMeta, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/elections/"+electionID+"/ballot",
		voterToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("ballot meta status=%d body=%s", status, string(raw))
	}

	candidateIDs := candidateIDsFromBallotMeta(t, ballotMeta)
	if len(candidateIDs) < 2 {
		t.Fatalf("expected at least 2 candidates, got %v", candidateIDs)
	}

	reqBody := map[string]any{
		"ranking": candidateIDs[:2],
	}

	status, firstResp, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/elections/"+electionID+"/ballots/submit",
		voterToken,
		reqBody,
		map[string]string{"Idempotency-Key": "idem-key-1"},
	)
	if status != http.StatusOK {
		t.Fatalf("first submit status=%d body=%s", status, string(raw))
	}
	firstBallotID := mustGetString(t, firstResp, "ballot_id")

	status, secondResp, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/elections/"+electionID+"/ballots/submit",
		voterToken,
		reqBody,
		map[string]string{"Idempotency-Key": "idem-key-1"},
	)
	if status != http.StatusOK {
		t.Fatalf("second same-key submit status=%d body=%s", status, string(raw))
	}
	secondBallotID := mustGetString(t, secondResp, "ballot_id")

	if firstBallotID != secondBallotID {
		t.Fatalf("expected same ballot id for same idempotency key, got %q vs %q", firstBallotID, secondBallotID)
	}

	status, errResp, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/elections/"+electionID+"/ballots/submit",
		voterToken,
		reqBody,
		map[string]string{"Idempotency-Key": "idem-key-2"},
	)
	if status != http.StatusConflict {
		t.Fatalf("new-key submit after accepted ballot should conflict; status=%d body=%s", status, string(raw))
	}
	code := responseErrorCode(t, errResp)
	if code != "already_submitted" {
		t.Fatalf("expected already_submitted, got %q; body=%s", code, string(raw))
	}
}

func TestIntegration_CloseCreatesTallyJobAndResultsVisibleOnlyAfterPublish(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_results")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	voterEmail := uniqueEmail("voter_results")
	voterPassword := "StrongPass123!"
	status, data, raw := doJSONRequest(t, env.client, http.MethodPost, env.server.URL+"/api/v1/auth/register", "", map[string]any{
		"email":    voterEmail,
		"password": voterPassword,
	})
	if status != http.StatusOK {
		t.Fatalf("register voter status=%d body=%s", status, string(raw))
	}
	voterToken := mustGetString(t, data, "access_token")

	electionID := createElectionAsAdmin(t, env, adminToken, "open")
	mustAction(t, env, adminToken, electionID, "schedule")
	mustAction(t, env, adminToken, electionID, "open")

	mustAction(t, env, adminToken, electionID, "close")

	var tallyJobs int
	err := env.pg.QueryRow(context.Background(), `
		SELECT count(*)
		FROM jobs
		WHERE election_id = $1::uuid
		  AND kind = 'tally'
		  AND status = 'queued'
	`, electionID).Scan(&tallyJobs)
	if err != nil {
		t.Fatalf("count tally jobs: %v", err)
	}
	if tallyJobs < 1 {
		t.Fatalf("expected queued tally job after close, got %d", tallyJobs)
	}

	status, errResp, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/elections/"+electionID+"/results",
		voterToken,
		nil,
	)
	if status != http.StatusForbidden {
		t.Fatalf("results before publish should be forbidden before publish; status=%d body=%s", status, string(raw))
	}
	if code := responseErrorCode(t, errResp); code != "not_published" {
		t.Fatalf("expected not_published before publish, got %q; body=%s", code, string(raw))
	}

	insertElectionResult(t, env.pg, electionID)

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/elections/"+electionID+"/actions/publish",
		adminToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", status, string(raw))
	}

	status, resultResp, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/elections/"+electionID+"/results",
		voterToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("results after publish status=%d body=%s", status, string(raw))
	}

	if _, ok := resultResp["winners"]; !ok {
		t.Fatalf("results response has no winners: %s", string(raw))
	}

	if v, ok := resultResp["metrics"]; ok && v != nil {
		t.Fatalf("metrics must be hidden for voter when show_aggregates=false: %s", string(raw))
	}
	if v, ok := resultResp["protocol"]; ok && v != nil {
		t.Fatalf("protocol must be hidden for voter when show_aggregates=false: %s", string(raw))
	}
	if v, ok := resultResp["params"]; ok && v != nil {
		t.Fatalf("params must be hidden for voter when show_aggregates=false: %s", string(raw))
	}
}