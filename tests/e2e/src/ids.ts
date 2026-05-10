import { randomUUID } from "node:crypto";

export function suffix(): string {
  return randomUUID().replaceAll("-", "").slice(0, 10);
}

export function idempotencyKey(): string {
  return randomUUID();
}

export function futureIso(minutesFromNow: number): string {
  return new Date(Date.now() + minutesFromNow * 60_000).toISOString();
}

export function daysFromNowIso(days: number): string {
  return new Date(Date.now() + days * 24 * 60 * 60_000).toISOString();
}