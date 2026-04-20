#!/usr/bin/env bash
set -euo pipefail

FILE="apps/backend/openapi/openapi.yaml"

grep -q '^  - name: Capabilities$' "$FILE"
grep -q '^  /api/v1/capabilities/tally-rules:$' "$FILE"
grep -q '^      tags: \[Capabilities\]$' "$FILE"
grep -q '^    TallyRuleInfo:$' "$FILE"
grep -q '^    TallyRuleListResponse:$' "$FILE"
grep -q '^        - ballot_formats$' "$FILE"
grep -q '^        - supports_election_tally$' "$FILE"
grep -q '^        - supports_experiment_runs$' "$FILE"
grep -q '^        - requires_committee_size$' "$FILE"
grep -q '^        - supports_quota_type$' "$FILE"
grep -q '^        - requires_approval_max_choices$' "$FILE"
grep -q '^        - supports_ranking_top_k$' "$FILE"
grep -q '^        - requires_score_range$' "$FILE"
grep -q '^  /api/v1/system/status:$' "$FILE"
grep -q '^      summary: Get backend, compute, and worker connectivity status$' "$FILE"
grep -q '^      required: \[backend, compute, worker, checked_at\]$' "$FILE"
grep -q '^        worker:$' "$FILE"
grep -q '^            enum: \[approval, ranking, score\]$' "$FILE"

echo "[openapi-capabilities-check] OK"