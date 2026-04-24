# MTBF validation approach

## Important note
The requirement "MTBF >= 500 hours" cannot be honestly confirmed by a short one-time test run.

## What is implemented
The project includes a long-run stability contour:
- `scripts/ci/run_mtbf_light_check.sh`

This contour:
- starts the prod profile
- periodically checks backend health
- periodically checks frontend availability
- periodically records system status
- stores the full observation log as CI artifacts

## Acceptance model
The requirement is treated as confirmed only after cumulative observation without critical service failure reaches at least 500 hours.

## Current evidence
Short and medium duration endurance runs may be used as partial evidence, but must not be described as full confirmation of 500 hours.

## Defensive wording for documentation
Recommended wording:
"The project includes an automated endurance validation contour for collecting MTBF evidence. At the current stage, short and medium duration runs confirm stable operation under the tested time window. Full confirmation of the 500-hour requirement requires continued accumulation of observation data."