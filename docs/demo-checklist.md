# Secure Voting Demo Checklist

## Before demo
- `go test ./...` is green in `apps/backend`
- `go vet ./...` is green in `apps/backend`
- `golangci-lint run` is green in `apps/backend`
- `npm run typecheck` is green in `apps/frontend`
- `npm run build` is green in `apps/frontend`
- `bash scripts/ci/run_demo_suite.sh` completes successfully

## Demo accounts
- admin bootstrap user exists
- researcher bootstrap user exists
- at least one ordinary voter is available for the live scenario

## Recommended live flow
1. Log in as admin
2. Create election with wizard
3. Open election
4. Log in as voter and submit ballot
5. Close election
6. Show that results are hidden before publish
7. Publish election
8. Show winners, protocol, metrics, and charts
9. Switch to researcher dashboard and show datasets / runs / monitoring

## Fallback plan
- keep `scripts/e2e_election_lifecycle.sh` ready as proof of reproducible flow
- keep `scripts/e2e_invite_only.sh` ready for access-control proof
- keep `scripts/e2e_vote_formats.sh` ready for ballot format proof
- keep compose logs saved after each rehearsal run
