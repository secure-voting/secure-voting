import type { TallyRuleInfo } from "../api/types";

export function mergeRuleItems(items: TallyRuleInfo[]): TallyRuleInfo[] {
  const map = new Map<string, TallyRuleInfo>();

  for (const item of items) {
    const existing = map.get(item.id);

    if (!existing) {
      map.set(item.id, {
        ...item,
        ballot_formats: Array.from(new Set(item.ballot_formats ?? [])),
      });
      continue;
    }

    map.set(item.id, {
      ...existing,
      label: existing.label || item.label,
      ballot_formats: Array.from(
        new Set([...(existing.ballot_formats ?? []), ...(item.ballot_formats ?? [])])
      ),
      supports_election_tally:
        existing.supports_election_tally || item.supports_election_tally,
      supports_experiment_runs:
        existing.supports_experiment_runs || item.supports_experiment_runs,
      requires_committee_size:
        existing.requires_committee_size || item.requires_committee_size,
      supports_quota_type:
        existing.supports_quota_type || item.supports_quota_type,
      requires_approval_max_choices:
        existing.requires_approval_max_choices || item.requires_approval_max_choices,
      supports_ranking_top_k:
        existing.supports_ranking_top_k || item.supports_ranking_top_k,
      requires_score_range:
        existing.requires_score_range || item.requires_score_range,
    });
  }

  return Array.from(map.values());
}