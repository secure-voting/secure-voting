export function tallyRuleLabel(value: string | null | undefined): string {
  const raw = String(value || "").trim();
  if (!raw) return "—";

  const normalized = raw.toLowerCase().replace(/_/g, "-");

  const fixed: Record<string, string> = {
    "approval-2": "Approval 2",
    "approval-3": "Approval 3",
    "plurality": "Plurality",
    "inverse-plurality": "Inverse Plurality",
    "inverse_plurality": "Inverse Plurality",
    "borda": "Borda",
    "inverse-borda": "Inverse Borda",
    "inverse_borda": "Inverse Borda",
    "black": "Black",
    "copeland-i": "Copeland I",
    "copeland-ii": "Copeland II",
    "copeland-iii": "Copeland III",
    "copeland_i": "Copeland I",
    "copeland_ii": "Copeland II",
    "copeland_iii": "Copeland III",
    "simpson": "Simpson",
    "minmax": "Minmax",
    "minimax": "Minmax",
    "hare": "Hare",
    "nanson": "Nanson",
    "coombs": "Coombs",
    "practical-condorcet": "Practical Condorcet",
    "practical_condorcet": "Practical Condorcet",
    "threshold": "Threshold",
  };

  if (fixed[raw]) return fixed[raw];
  if (fixed[normalized]) return fixed[normalized];

  return raw
    .replace(/[_-]+/g, " ")
    .split(" ")
    .filter(Boolean)
    .map((part) => {
      const upper = part.toUpperCase();
      if (["I", "II", "III", "IV", "V"].includes(upper)) return upper;
      if (/^\d+$/.test(part)) return part;
      return part.charAt(0).toUpperCase() + part.slice(1).toLowerCase();
    })
    .join(" ");
}