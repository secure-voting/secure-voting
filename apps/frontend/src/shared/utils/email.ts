export function parseEmailsFromText(text: string) {
  return text
    .split(/[\n,;]+/g)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function uniqueEmails(emails: string[]) {
  const out: string[] = [];
  const seen = new Set<string>();

  for (const email of emails) {
    const normalized = email.toLowerCase();
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    out.push(email);
  }

  return out;
}

export function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}