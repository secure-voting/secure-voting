# Backup policy

## Scope
The system stores relational data in PostgreSQL and non-relational data in MongoDB. Redis is treated as a transient operational store and is not the primary recovery source.

## Schedule
- Daily full PostgreSQL backup
- Daily full MongoDB backup
- Daily checksum generation for every backup artifact
- Daily restore verification in a dedicated non-production contour

## Retention
- 7 daily restore points
- 4 weekly restore points
- 6 monthly restore points

## Storage layout
Backups are stored under `.backups/<UTC timestamp>/`.

Each backup bundle contains:
- `postgres_secure_voting.dump`
- `postgres_secure_voting.dump.sha256`
- `mongo_secure_voting.archive.gz`
- `mongo_secure_voting.archive.gz.sha256`
- `manifest.txt`
- `files.txt`

## Restore validation
Restore validation is executed by `scripts/ci/run_restore_check.sh`.
Acceptance criteria:
- PostgreSQL marker row is restored
- MongoDB marker document is restored
- Restore duration is less than or equal to 600 seconds

## Rotation
Rotation is executed by `scripts/ops/prune_old_backups.sh`.

## Notes
This policy confirms the operational procedure and the automated verification contour. It does not replace off-host storage or enterprise backup orchestration.