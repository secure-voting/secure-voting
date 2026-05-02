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

	rdb, err := db.NewRedisClient(
		bootCtx,
		cfg.RedisAddr,
		cfg.RedisPassword,
		cfg.RedisTLS,
		cfg.RedisTLSCA,
		cfg.RedisTLSServerName,
	)
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
		"title":          "Integration election",
		"description":    "integration-test",
		"start_at":       now.Add(1 * time.Hour).Format(time.RFC3339),
		"end_at":         now.Add(2 * time.Hour).Format(time.RFC3339),
		"tally_rule":     "borda",
		"ballot_format":  "ranking",
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

func TestIntegration_SingleSessionLoginReplaceRevokesPreviousSession(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("single_session")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")

	status, firstLogin, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/auth/login",
		"",
		map[string]any{
			"email":    email,
			"password": password,
		},
	)
	if status != http.StatusOK {
		t.Fatalf("first login status=%d body=%s", status, string(raw))
	}

	firstAccessToken := mustGetString(t, firstLogin, "access_token")
	firstRefreshToken := mustGetString(t, firstLogin, "refresh_token")

	if firstAccessToken == "" {
		t.Fatal("first access token is empty")
	}
	if firstRefreshToken == "" {
		t.Fatal("first refresh token is empty")
	}

	status, secondLoginConflict, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/auth/login",
		"",
		map[string]any{
			"email":    email,
			"password": password,
		},
	)
	if status != http.StatusConflict {
		t.Fatalf("second login without replacement should return 409; status=%d body=%s", status, string(raw))
	}
	if code := responseErrorCode(t, secondLoginConflict); code != "active_session_exists" {
		t.Fatalf("expected active_session_exists, got %q; body=%s", code, string(raw))
	}

	status, secondLogin, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/auth/login",
		"",
		map[string]any{
			"email":                    email,
			"password":                 password,
			"replace_existing_session": true,
		},
	)
	if status != http.StatusOK {
		t.Fatalf("second login with replacement status=%d body=%s", status, string(raw))
	}

	secondAccessToken := mustGetString(t, secondLogin, "access_token")
	secondRefreshToken := mustGetString(t, secondLogin, "refresh_token")

	if secondAccessToken == "" {
		t.Fatal("second access token is empty")
	}
	if secondRefreshToken == "" {
		t.Fatal("second refresh token is empty")
	}
	if secondAccessToken == firstAccessToken {
		t.Fatal("replacement login returned the same access token")
	}
	if secondRefreshToken == firstRefreshToken {
		t.Fatal("replacement login returned the same refresh token")
	}

	status, oldMe, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/auth/me",
		firstAccessToken,
		nil,
	)
	if status != http.StatusUnauthorized {
		t.Fatalf("old access token should be unauthorized after replacement; status=%d body=%s", status, string(raw))
	}
	if code := responseErrorCode(t, oldMe); code != "unauthorized" {
		t.Fatalf("expected unauthorized for old access token, got %q; body=%s", code, string(raw))
	}

	status, oldRefresh, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/auth/refresh",
		"",
		map[string]any{
			"refresh_token": firstRefreshToken,
		},
	)
	if status != http.StatusUnauthorized {
		t.Fatalf("old refresh token should be unauthorized after replacement; status=%d body=%s", status, string(raw))
	}
	if code := responseErrorCode(t, oldRefresh); code != "unauthorized" {
		t.Fatalf("expected unauthorized for old refresh token, got %q; body=%s", code, string(raw))
	}

	status, newMe, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/auth/me",
		secondAccessToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("new access token should work; status=%d body=%s", status, string(raw))
	}

	if gotEmail := mustGetString(t, newMe, "email"); gotEmail != email {
		t.Fatalf("unexpected current user email: got=%q want=%q", gotEmail, email)
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

func TestIntegration_MeReturnsProfileFields(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("profile_me")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")

	_, err := env.pg.Exec(context.Background(), `
		UPDATE users
		SET full_name = $2, phone = $3
		WHERE lower(email) = lower($1)
	`, email, "Иван Иванов", "+79990000000")
	if err != nil {
		t.Fatalf("update profile fields in db: %v", err)
	}

	token := loginAndGetToken(t, env, email, password)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/auth/me",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("me status=%d body=%s", status, string(raw))
	}

	if got := mustGetString(t, data, "id"); got == "" {
		t.Fatalf("expected non-empty id, body=%s", string(raw))
	}
	if got := mustGetString(t, data, "email"); strings.ToLower(got) != strings.ToLower(email) {
		t.Fatalf("unexpected email=%q body=%s", got, string(raw))
	}
	if got := mustGetString(t, data, "role"); got != "voter" {
		t.Fatalf("unexpected role=%q body=%s", got, string(raw))
	}

	fullName, ok := data["full_name"].(string)
	if !ok || fullName != "Иван Иванов" {
		t.Fatalf("unexpected full_name=%#v body=%s", data["full_name"], string(raw))
	}

	phone, ok := data["phone"].(string)
	if !ok || phone != "+79990000000" {
		t.Fatalf("unexpected phone=%#v body=%s", data["phone"], string(raw))
	}
}

func TestIntegration_UpdateProfile(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("profile_patch")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")

	token := loginAndGetToken(t, env, email, password)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPatch,
		env.server.URL+"/api/v1/auth/profile",
		token,
		map[string]any{
			"full_name": "Петр Петров",
			"phone":     "+7 (999) 111-22-33",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("patch profile status=%d body=%s", status, string(raw))
	}

	fullName, ok := data["full_name"].(string)
	if !ok || fullName != "Петр Петров" {
		t.Fatalf("unexpected full_name=%#v body=%s", data["full_name"], string(raw))
	}

	phone, ok := data["phone"].(string)
	if !ok || phone != "+7 (999) 111-22-33" {
		t.Fatalf("unexpected phone=%#v body=%s", data["phone"], string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/auth/me",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("me after patch status=%d body=%s", status, string(raw))
	}

	fullName, ok = data["full_name"].(string)
	if !ok || fullName != "Петр Петров" {
		t.Fatalf("unexpected full_name after patch=%#v body=%s", data["full_name"], string(raw))
	}

	phone, ok = data["phone"].(string)
	if !ok || phone != "+7 (999) 111-22-33" {
		t.Fatalf("unexpected phone after patch=%#v body=%s", data["phone"], string(raw))
	}

	var dbFullName, dbPhone *string
	err := env.pg.QueryRow(context.Background(), `
		SELECT full_name, phone
		FROM users
		WHERE lower(email) = lower($1)
	`, email).Scan(&dbFullName, &dbPhone)
	if err != nil {
		t.Fatalf("select updated profile: %v", err)
	}

	if dbFullName == nil || *dbFullName != "Петр Петров" {
		t.Fatalf("unexpected db full_name=%#v", dbFullName)
	}
	if dbPhone == nil || *dbPhone != "+7 (999) 111-22-33" {
		t.Fatalf("unexpected db phone=%#v", dbPhone)
	}
}

func notificationItemsFromListResponse(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()

	itemsV, ok := data["items"]
	if !ok {
		t.Fatalf("missing items in response: %#v", data)
	}

	items, ok := itemsV.([]any)
	if !ok {
		t.Fatalf("items is not array: %#v", itemsV)
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("notification item is not object: %#v", it)
		}
		out = append(out, obj)
	}
	return out
}

func findNotificationByID(items []map[string]any, id string) (map[string]any, bool) {
	for _, it := range items {
		gotID, _ := it["id"].(string)
		if gotID == id {
			return it, true
		}
	}
	return nil, false
}

func TestIntegration_NotificationsLifecycle(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("notif_lifecycle")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")
	token := loginAndGetToken(t, env, email, password)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/notifications",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("initial notifications list status=%d body=%s", status, string(raw))
	}

	status, created, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/notifications",
		token,
		map[string]any{
			"title":        "Новый результат",
			"message":      "Результаты голосования опубликованы.",
			"details":      "Тестовое уведомление",
			"action_label": "Открыть",
			"action_to":    "/results/test",
			"kind":         "success",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("create notification status=%d body=%s", status, string(raw))
	}

	notificationID := mustGetString(t, created, "id")
	if title := mustGetString(t, created, "title"); title != "Новый результат" {
		t.Fatalf("unexpected title=%q body=%s", title, string(raw))
	}
	if message := mustGetString(t, created, "message"); message != "Результаты голосования опубликованы." {
		t.Fatalf("unexpected message=%q body=%s", message, string(raw))
	}
	if kind := mustGetString(t, created, "kind"); kind != "success" {
		t.Fatalf("unexpected kind=%q body=%s", kind, string(raw))
	}
	if read, ok := created["read"].(bool); !ok || read {
		t.Fatalf("expected unread created notification, got read=%#v body=%s", created["read"], string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/notifications",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("notifications list after create status=%d body=%s", status, string(raw))
	}

	items := notificationItemsFromListResponse(t, data)
	item, ok := findNotificationByID(items, notificationID)
	if !ok {
		t.Fatalf("created notification %q not found in list body=%s", notificationID, string(raw))
	}
	if read, ok := item["read"].(bool); !ok || read {
		t.Fatalf("expected unread notification in list, got read=%#v body=%s", item["read"], string(raw))
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/notifications/"+notificationID+"/read",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("mark read status=%d body=%s", status, string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/notifications",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("notifications list after mark read status=%d body=%s", status, string(raw))
	}

	items = notificationItemsFromListResponse(t, data)
	item, ok = findNotificationByID(items, notificationID)
	if !ok {
		t.Fatalf("notification %q not found after mark read body=%s", notificationID, string(raw))
	}
	if read, ok := item["read"].(bool); !ok || !read {
		t.Fatalf("expected read notification after mark read, got read=%#v body=%s", item["read"], string(raw))
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodDelete,
		env.server.URL+"/api/v1/notifications/"+notificationID,
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("delete notification status=%d body=%s", status, string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/notifications",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("notifications list after delete status=%d body=%s", status, string(raw))
	}

	items = notificationItemsFromListResponse(t, data)
	if _, ok := findNotificationByID(items, notificationID); ok {
		t.Fatalf("deleted notification %q still present body=%s", notificationID, string(raw))
	}
}

func TestIntegration_NotificationsMarkAllReadAndClearAll(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("notif_bulk")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")
	token := loginAndGetToken(t, env, email, password)

	status, _, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/notifications",
		token,
		map[string]any{
			"title":   "Уведомление 1",
			"message": "Первое тестовое уведомление",
			"kind":    "info",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("create first notification status=%d body=%s", status, string(raw))
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/notifications",
		token,
		map[string]any{
			"title":   "Уведомление 2",
			"message": "Второе тестовое уведомление",
			"kind":    "warning",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("create second notification status=%d body=%s", status, string(raw))
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/notifications/read-all",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("mark all read status=%d body=%s", status, string(raw))
	}

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/notifications",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("notifications list after mark all read status=%d body=%s", status, string(raw))
	}

	items := notificationItemsFromListResponse(t, data)
	if len(items) == 0 {
		t.Fatalf("expected non-empty notifications list body=%s", string(raw))
	}
	for _, item := range items {
		read, ok := item["read"].(bool)
		if !ok || !read {
			t.Fatalf("expected all notifications to be read, item=%#v body=%s", item, string(raw))
		}
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodDelete,
		env.server.URL+"/api/v1/notifications",
		token,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("clear all notifications status=%d body=%s", status, string(raw))
	}

	var count int
	err := env.pg.QueryRow(context.Background(), `
		SELECT count(*)
		FROM notifications n
		JOIN users u ON u.id = n.user_id
		WHERE lower(u.email) = lower($1)
	`, email).Scan(&count)
	if err != nil {
		t.Fatalf("count notifications after clear all: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 notifications in db after clear all, got %d", count)
	}
}

func adminUserItemsFromListResponse(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()

	itemsV, ok := data["items"]
	if !ok {
		t.Fatalf("missing items: %#v", data)
	}

	items, ok := itemsV.([]any)
	if !ok {
		t.Fatalf("items is not array: %#v", itemsV)
	}

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("item is not object: %#v", it)
		}
		out = append(out, obj)
	}
	return out
}

func findAdminUserByEmail(t *testing.T, items []map[string]any, email string) (map[string]any, bool) {
	t.Helper()

	for _, item := range items {
		gotEmail, _ := item["email"].(string)
		if strings.EqualFold(strings.TrimSpace(gotEmail), strings.TrimSpace(email)) {
			return item, true
		}
	}
	return nil, false
}

func getUserIDByEmail(t *testing.T, pg *pgxpool.Pool, email string) string {
	t.Helper()

	var id string
	err := pg.QueryRow(context.Background(),
		`SELECT id::text FROM users WHERE lower(email)=lower($1)`,
		email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("get user id: %v", err)
	}
	return id
}

func TestIntegration_AdminUsersListAndUpdateRole(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_users_admin")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	targetEmail := uniqueEmail("admin_users_target")
	targetPassword := "StrongPass123!"
	upsertUser(t, env.pg, targetEmail, targetPassword, "voter")
	targetUserID := getUserIDByEmail(t, env.pg, targetEmail)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/users?limit=200",
		adminToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("admin users list status=%d body=%s", status, string(raw))
	}

	items := adminUserItemsFromListResponse(t, data)
	targetItem, ok := findAdminUserByEmail(t, items, targetEmail)
	if !ok {
		t.Fatalf("target user not found in list body=%s", string(raw))
	}
	if gotRole, _ := targetItem["role"].(string); gotRole != "voter" {
		t.Fatalf("expected initial role voter, got %q body=%s", gotRole, string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPatch,
		env.server.URL+"/api/v1/admin/users/"+targetUserID+"/role",
		adminToken,
		map[string]any{
			"role": "researcher",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("update role status=%d body=%s", status, string(raw))
	}

	if gotRole, _ := data["role"].(string); gotRole != "researcher" {
		t.Fatalf("expected updated role researcher, got %q body=%s", gotRole, string(raw))
	}

	dbRole := getUserRole(t, env.pg, targetEmail)
	if dbRole != "researcher" {
		t.Fatalf("expected db role researcher, got %q", dbRole)
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/users?limit=200",
		adminToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("admin users list after update status=%d body=%s", status, string(raw))
	}

	items = adminUserItemsFromListResponse(t, data)
	targetItem, ok = findAdminUserByEmail(t, items, targetEmail)
	if !ok {
		t.Fatalf("target user not found after update body=%s", string(raw))
	}
	if gotRole, _ := targetItem["role"].(string); gotRole != "researcher" {
		t.Fatalf("expected listed role researcher, got %q body=%s", gotRole, string(raw))
	}
}

func TestIntegration_AdminUsersForbiddenForNonAdmin(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_users_owner")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")

	userEmail := uniqueEmail("admin_users_regular")
	userPassword := "StrongPass123!"
	upsertUser(t, env.pg, userEmail, userPassword, "voter")
	userToken := loginAndGetToken(t, env, userEmail, userPassword)

	targetEmail := uniqueEmail("admin_users_target_regular")
	targetPassword := "StrongPass123!"
	upsertUser(t, env.pg, targetEmail, targetPassword, "voter")
	targetUserID := getUserIDByEmail(t, env.pg, targetEmail)

	status, _, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/users",
		userToken,
		nil,
	)
	if status != http.StatusForbidden {
		t.Fatalf("non-admin list should be forbidden; status=%d body=%s", status, string(raw))
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPatch,
		env.server.URL+"/api/v1/admin/users/"+targetUserID+"/role",
		userToken,
		map[string]any{
			"role": "researcher",
		},
	)
	if status != http.StatusForbidden {
		t.Fatalf("non-admin update should be forbidden; status=%d body=%s", status, string(raw))
	}
}

func TestIntegration_AdminUsersSelfRoleChangeForbidden(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_users_self")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)
	adminUserID := getUserIDByEmail(t, env.pg, adminEmail)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodPatch,
		env.server.URL+"/api/v1/admin/users/"+adminUserID+"/role",
		adminToken,
		map[string]any{
			"role": "voter",
		},
	)
	if status != http.StatusBadRequest {
		t.Fatalf("self role change should be bad request; status=%d body=%s", status, string(raw))
	}

	code := responseErrorCode(t, data)
	if code != "bad_request" {
		t.Fatalf("expected bad_request, got %q body=%s", code, string(raw))
	}

	errObj, _ := data["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if msg != "cannot change own role" {
		t.Fatalf("expected message %q, got %q body=%s", "cannot change own role", msg, string(raw))
	}

	dbRole := getUserRole(t, env.pg, adminEmail)
	if dbRole != "admin" {
		t.Fatalf("expected admin role to stay unchanged, got %q", dbRole)
	}
}

func TestIntegration_AuthRateLimit_Login(t *testing.T) {
	t.Setenv("AUTH_RATE_LIMIT", "2")
	t.Setenv("AUTH_RATE_LIMIT_TTL", "1m")

	env := newIntegrationEnv(t)

	ip := "203.0.113.10"

	for i := 0; i < 2; i++ {
		status, data, raw := doJSONRequestWithHeaders(
			t,
			env.client,
			http.MethodPost,
			env.server.URL+"/api/v1/auth/login",
			"",
			map[string]any{
				"email":    "nobody@example.com",
				"password": "wrong-password",
			},
			map[string]string{
				"X-Forwarded-For": ip,
			},
		)
		if status != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d body=%s", i+1, status, string(raw))
		}
		if code := responseErrorCode(t, data); code != "unauthorized" {
			t.Fatalf("attempt %d expected unauthorized, got %q body=%s", i+1, code, string(raw))
		}
	}

	status, data, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/auth/login",
		"",
		map[string]any{
			"email":    "nobody@example.com",
			"password": "wrong-password",
		},
		map[string]string{
			"X-Forwarded-For": ip,
		},
	)
	if status != http.StatusTooManyRequests {
		t.Fatalf("third attempt expected 429, got %d body=%s", status, string(raw))
	}
	if code := responseErrorCode(t, data); code != "rate_limited" {
		t.Fatalf("expected rate_limited, got %q body=%s", code, string(raw))
	}
}

func TestIntegration_WriteRateLimit_NotificationsCreate(t *testing.T) {
	t.Setenv("WRITE_RATE_LIMIT", "2")
	t.Setenv("WRITE_RATE_LIMIT_TTL", "1m")

	env := newIntegrationEnv(t)

	email := uniqueEmail("write_rl_notif")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")
	token := loginAndGetToken(t, env, email, password)

	ip := "203.0.113.20"

	for i := 0; i < 2; i++ {
		status, data, raw := doJSONRequestWithHeaders(
			t,
			env.client,
			http.MethodPost,
			env.server.URL+"/api/v1/notifications",
			token,
			map[string]any{
				"title":   "Ограничение записи",
				"message": "Тест write rate limit",
				"kind":    "info",
			},
			map[string]string{
				"X-Forwarded-For": ip,
			},
		)
		if status != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d body=%s", i+1, status, string(raw))
		}
		if got := mustGetString(t, data, "title"); got != "Ограничение записи" {
			t.Fatalf("attempt %d unexpected title=%q body=%s", i+1, got, string(raw))
		}
	}

	status, data, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodPost,
		env.server.URL+"/api/v1/notifications",
		token,
		map[string]any{
			"title":   "Ограничение записи",
			"message": "Тест write rate limit",
			"kind":    "info",
		},
		map[string]string{
			"X-Forwarded-For": ip,
		},
	)
	if status != http.StatusTooManyRequests {
		t.Fatalf("third attempt expected 429, got %d body=%s", status, string(raw))
	}
	if code := responseErrorCode(t, data); code != "rate_limited" {
		t.Fatalf("expected rate_limited, got %q body=%s", code, string(raw))
	}
}

func TestIntegration_SecurityHeaders_Health(t *testing.T) {
	env := newIntegrationEnv(t)

	req, err := http.NewRequest(http.MethodGet, env.server.URL+"/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options: %q", got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("unexpected X-Frame-Options: %q", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("unexpected Referrer-Policy: %q", got)
	}
	if got := resp.Header.Get("X-XSS-Protection"); got != "0" {
		t.Fatalf("unexpected X-XSS-Protection: %q", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'" {
		t.Fatalf("unexpected Content-Security-Policy: %q", got)
	}
}

func TestIntegration_AdminTrustedCIDRs(t *testing.T) {
	t.Setenv("ADMIN_TRUSTED_CIDRS", "203.0.113.0/24")

	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_trusted")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	status, _, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/users",
		adminToken,
		nil,
		map[string]string{
			"X-Forwarded-For": "203.0.113.10",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("trusted admin request expected 200, got %d body=%s", status, string(raw))
	}

	status, data, raw := doJSONRequestWithHeaders(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/users",
		adminToken,
		nil,
		map[string]string{
			"X-Forwarded-For": "198.51.100.10",
		},
	)
	if status != http.StatusForbidden {
		t.Fatalf("untrusted admin request expected 403, got %d body=%s", status, string(raw))
	}

	if code := responseErrorCode(t, data); code != "forbidden_network" {
		t.Fatalf("expected forbidden_network, got %q body=%s", code, string(raw))
	}
}

func TestIntegration_AdminSettingsGetAndUpdate(t *testing.T) {
	env := newIntegrationEnv(t)

	adminEmail := uniqueEmail("admin_settings")
	adminPassword := "StrongPass123!"
	upsertUser(t, env.pg, adminEmail, adminPassword, "admin")
	adminToken := loginAndGetToken(t, env, adminEmail, adminPassword)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/settings",
		adminToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("initial settings get status=%d body=%s", status, string(raw))
	}

	if got, _ := data["tls_mode"].(string); got != "disabled" {
		t.Fatalf("expected default tls_mode=disabled, got %q body=%s", got, string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPut,
		env.server.URL+"/api/v1/admin/settings",
		adminToken,
		map[string]any{
			"public_base_url":       "https://vote.example.com",
			"tls_mode":              "lets_encrypt",
			"tls_domain":            "vote.example.com",
			"tls_contact_email":     "admin@example.com",
			"backup_enabled":        true,
			"backup_schedule":       "daily 02:00",
			"backup_retention_days": 7,
			"database_host":         "postgres-db",
			"database_name":         "secure_voting",
		},
	)
	if status != http.StatusOK {
		t.Fatalf("settings update status=%d body=%s", status, string(raw))
	}

	if got, _ := data["tls_mode"].(string); got != "lets_encrypt" {
		t.Fatalf("expected tls_mode lets_encrypt, got %q body=%s", got, string(raw))
	}
	if got, _ := data["public_base_url"].(string); got != "https://vote.example.com" {
		t.Fatalf("expected public_base_url saved, got %q body=%s", got, string(raw))
	}
	if got, ok := data["backup_enabled"].(bool); !ok || !got {
		t.Fatalf("expected backup_enabled=true, got %#v body=%s", data["backup_enabled"], string(raw))
	}

	status, data, raw = doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/settings",
		adminToken,
		nil,
	)
	if status != http.StatusOK {
		t.Fatalf("settings get after update status=%d body=%s", status, string(raw))
	}

	if got, _ := data["database_name"].(string); got != "secure_voting" {
		t.Fatalf("expected database_name secure_voting, got %q body=%s", got, string(raw))
	}

	var (
		publicBaseURL   *string
		tlsMode         string
		tlsDomain       *string
		tlsContactEmail *string
		backupEnabled   bool
		backupSchedule  *string
		backupRetention *int
		databaseHost    *string
		databaseName    *string
	)
	err := env.pg.QueryRow(context.Background(), `
		SELECT
			public_base_url,
			tls_mode,
			tls_domain,
			tls_contact_email,
			backup_enabled,
			backup_schedule,
			backup_retention_days,
			database_host,
			database_name
		FROM admin_settings
		WHERE id = 1
	`).Scan(
		&publicBaseURL,
		&tlsMode,
		&tlsDomain,
		&tlsContactEmail,
		&backupEnabled,
		&backupSchedule,
		&backupRetention,
		&databaseHost,
		&databaseName,
	)
	if err != nil {
		t.Fatalf("select admin_settings: %v", err)
	}

	if publicBaseURL == nil || *publicBaseURL != "https://vote.example.com" {
		t.Fatalf("unexpected db public_base_url=%#v", publicBaseURL)
	}
	if tlsMode != "lets_encrypt" {
		t.Fatalf("unexpected db tls_mode=%q", tlsMode)
	}
	if tlsDomain == nil || *tlsDomain != "vote.example.com" {
		t.Fatalf("unexpected db tls_domain=%#v", tlsDomain)
	}
	if tlsContactEmail == nil || *tlsContactEmail != "admin@example.com" {
		t.Fatalf("unexpected db tls_contact_email=%#v", tlsContactEmail)
	}
	if !backupEnabled {
		t.Fatal("expected db backup_enabled=true")
	}
	if backupSchedule == nil || *backupSchedule != "daily 02:00" {
		t.Fatalf("unexpected db backup_schedule=%#v", backupSchedule)
	}
	if backupRetention == nil || *backupRetention != 7 {
		t.Fatalf("unexpected db backup_retention_days=%#v", backupRetention)
	}
	if databaseHost == nil || *databaseHost != "postgres-db" {
		t.Fatalf("unexpected db database_host=%#v", databaseHost)
	}
	if databaseName == nil || *databaseName != "secure_voting" {
		t.Fatalf("unexpected db database_name=%#v", databaseName)
	}
}

func TestIntegration_AdminSettingsForbiddenForNonAdmin(t *testing.T) {
	env := newIntegrationEnv(t)

	userEmail := uniqueEmail("admin_settings_regular")
	userPassword := "StrongPass123!"
	upsertUser(t, env.pg, userEmail, userPassword, "voter")
	userToken := loginAndGetToken(t, env, userEmail, userPassword)

	status, _, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/admin/settings",
		userToken,
		nil,
	)
	if status != http.StatusForbidden {
		t.Fatalf("non-admin settings get should be forbidden; status=%d body=%s", status, string(raw))
	}

	status, _, raw = doJSONRequest(
		t,
		env.client,
		http.MethodPut,
		env.server.URL+"/api/v1/admin/settings",
		userToken,
		map[string]any{
			"public_base_url":       "https://vote.example.com",
			"tls_mode":              "disabled",
			"tls_domain":            "",
			"tls_contact_email":     "",
			"backup_enabled":        false,
			"backup_schedule":       "",
			"backup_retention_days": nil,
			"database_host":         "",
			"database_name":         "",
		},
	)
	if status != http.StatusForbidden {
		t.Fatalf("non-admin settings update should be forbidden; status=%d body=%s", status, string(raw))
	}
}

func TestIntegration_CapabilitiesTallyRules_ComputeUnavailable(t *testing.T) {
	env := newIntegrationEnv(t)

	email := uniqueEmail("capabilities_user")
	password := "StrongPass123!"
	upsertUser(t, env.pg, email, password, "voter")
	token := loginAndGetToken(t, env, email, password)

	status, data, raw := doJSONRequest(
		t,
		env.client,
		http.MethodGet,
		env.server.URL+"/api/v1/capabilities/tally-rules",
		token,
		nil,
	)
	if status != http.StatusInternalServerError {
		t.Fatalf("capabilities tally-rules expected 500 when compute is disabled; status=%d body=%s", status, string(raw))
	}

	code := responseErrorCode(t, data)
	if code != "internal_error" {
		t.Fatalf("expected internal_error, got %q body=%s", code, string(raw))
	}

	errObj, ok := data["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object body=%s", string(raw))
	}

	msg, _ := errObj["message"].(string)
	if msg != "list tally rules failed" {
		t.Fatalf("expected message %q, got %q body=%s", "list tally rules failed", msg, string(raw))
	}
}
