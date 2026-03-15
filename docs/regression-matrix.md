# Secure Voting — regression matrix for CI and demo readiness

## 1. Fast checks
- Backend compilation and unit tests: `cd apps/backend && go test ./...`
- Backend static analysis: `cd apps/backend && go vet ./...`
- Backend lint: `cd apps/backend && golangci-lint run`
- Frontend type safety: `cd apps/frontend && npm run typecheck`
- Frontend production build: `cd apps/frontend && npm run build`
- OpenAPI validation: `python3 -m openapi_spec_validator`

## 2. Regressions that must be covered automatically

### Auth and access control
- Public register cannot create `admin`
- Public register cannot create `researcher`
- Invite-only election requires accepted invite for ballot meta access
- Invite-only election requires accepted invite for results access
- Ordinary users cannot see draft elections
- Owner keeps access to own election

### Ballots and voting flow
- Ballot submit without `Idempotency-Key` returns `missing_idempotency_key`
- Parallel or repeated submit returns `idempotency_in_progress` where applicable
- Re-submit after accepted ballot returns `already_submitted`
- Ranking ballot rejects duplicates
- Ranking ballot rejects gaps and invalid top-k selections
- Approval ballot enforces max choices
- Score ballot enforces min/max/step and allow-skip rules

### Results lifecycle
- Results before publish return `403` with `not_published`
- `close` creates tally job
- Published election exposes winners, metrics and protocol
- Frontend understands actual backend business codes such as `not_active`

### Experiments and datasets
- Researcher sees only own experiments
- Admin sees all experiments
- Dataset generate validates format and score rules before insert
- Dataset import returns `invalid_candidates` when ballots exist without candidates
- Dataset list is stable and sorted by `created_at DESC`
- Oversized dataset upload returns `413 payload_too_large`

### API contract stability
- List endpoints return `items: []` and never `null`
- Tally rule aliases normalize to canonical values
- OpenAPI spec matches real handlers closely enough for frontend generation and manual validation

## 3. Demo suite

### Automated demo scripts
- `scripts/e2e_election_lifecycle.sh`
- `scripts/e2e_invite_only.sh`
- `scripts/e2e_vote_formats.sh`
- `scripts/e2e_smoke.sh`
- `scripts/e2e_smoke_experiment.sh` after env is stabilized for CI

### Manual demo flow
1. Login as bootstrap admin
2. Create election through admin wizard
3. Schedule or open election
4. Login as voter and submit a ballot
5. Show that results are unavailable before publish
6. Close and publish election
7. Show winners, metrics and protocol timeline
8. Switch to researcher dashboard and show datasets, experiments, runs and monitoring

## 4. Remaining work to reach demo-ready status
- Stabilize one reproducible CI environment file for docker compose
- Decide whether experiment demo is mandatory for the intermediate presentation
- Add one root command for local pre-push verification
- Add artifacts upload in CI for logs if an e2e script fails
- Freeze a short manual demo script with exact accounts and commands
