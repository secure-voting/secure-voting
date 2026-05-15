import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../shared/api/client";
import type { AuditLogItem } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { ActionMenu } from "../../shared/ui/ActionMenu";
import {
  downloadCsvFile,
  downloadJsonFile,
  downloadPdfTextFile,
  downloadXlsxFile,
} from "../../shared/utils/export";
import { formatDateTime } from "../../shared/utils/dateTime";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function compact(value: unknown) {
  if (value == null) return "";
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function shortId(value: unknown) {
  const raw = typeof value === "string" || typeof value === "number" ? String(value).trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
}

function normalizeSearch(value: unknown) {
  return String(value ?? "").trim().toLowerCase();
}

function dateStartIso(value: string) {
  if (!value.trim()) return "";
  const ms = Date.parse(`${value}T00:00:00`);
  return Number.isFinite(ms) ? new Date(ms).toISOString() : "";
}

function dateEndIso(value: string) {
  if (!value.trim()) return "";
  const ms = Date.parse(`${value}T23:59:59.999`);
  return Number.isFinite(ms) ? new Date(ms).toISOString() : "";
}

function detailsOf(item: AuditLogItem): Record<string, unknown> {
  const raw = (item as Record<string, unknown>).details;
  return isObject(raw) ? raw : {};
}

function auditEventLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    user_registered: "Пользователь зарегистрирован",
    user_logged_in: "Пользователь вошел в систему",
    user_logged_out: "Пользователь вышел из системы",
    user_password_changed: "Пароль пользователя изменен",
    user_profile_updated: "Профиль пользователя обновлен",
    auth_token_refreshed: "Токены сессии обновлены",

    election_created: "Голосование создано",
    election_rules_updated: "Правила голосования обновлены",
    election_scheduled: "Голосование запланировано",
    election_opened: "Голосование открыто",
    election_paused: "Голосование приостановлено",
    election_resumed: "Голосование возобновлено",
    election_closed: "Голосование завершено",
    election_published: "Результаты опубликованы",

    invite_created: "Приглашение создано",
    invite_accepted: "Приглашение принято",
    invite_registration_required: "Для приглашения требуется регистрация",

    ballot_submitted: "Бюллетень отправлен",
    experiment_created: "Эксперимент создан",
    experiment_run_created: "Запуск эксперимента создан",
    dataset_imported: "Набор данных импортирован",
    dataset_generated: "Набор данных сгенерирован",
  };

  return labels[raw] || raw || "Событие";
}

function targetTypeLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    user: "Пользователь",
    auth_session: "Сессия",
    election: "Голосование",
    election_invite: "Приглашение",
    ballot: "Бюллетень",
    experiment: "Эксперимент",
    experiment_run: "Запуск эксперимента",
    dataset: "Набор данных",
    job: "Задача",
  };

  return labels[raw] || raw || "Объект";
}

function auditMainDescription(item: AuditLogItem) {
  const details = detailsOf(item);
  const targetType = details.target_type;
  const targetID = details.target_id;
  const after = isObject(details.after) ? details.after : null;
  const before = isObject(details.before) ? details.before : null;

  const email =
    (after && typeof after.email === "string" ? after.email : "") ||
    (before && typeof before.email === "string" ? before.email : "");

  const title =
    (after && typeof after.title === "string" ? after.title : "") ||
    (before && typeof before.title === "string" ? before.title : "");

  const status =
    (after && typeof after.status === "string" ? after.status : "") ||
    (before && typeof before.status === "string" ? before.status : "");

  const parts: string[] = [];

  if (title) parts.push(title);
  if (email) parts.push(email);
  if (status) parts.push(`статус: ${status}`);

  if (typeof targetID === "string" && targetID.trim()) {
    parts.push(`${targetTypeLabel(targetType)} ${shortId(targetID)}`);
  }

  return parts.length > 0 ? parts.join(" · ") : "Подробности доступны в техническом блоке";
}

function auditCsvRows(items: AuditLogItem[]) {
  return items.map((item) => {
    const details = detailsOf(item);

    return {
      event: auditEventLabel(item.event_type),
      occurred_at: compact(item.occurred_at),
      actor_user_id: compact((item as Record<string, unknown>).actor_user_id),
      target_type: compact(details.target_type),
      target_id: compact(details.target_id),
      id: compact(item.id),
      event_type: compact(item.event_type),
      details: compact(details),
    };
  });
}

function buildAuditReportText(
  items: AuditLogItem[],
  selected: AuditLogItem | null,
  filters: {
    eventType: string;
    actorUserId: string;
    since: string;
    until: string;
  }
) {
  const lines: string[] = [];

  lines.push("Отчет по журналу аудита");
  lines.push("");

  lines.push("Фильтры:");
  lines.push(`- тип события: ${filters.eventType || "—"}`);
  lines.push(`- ID пользователя: ${filters.actorUserId || "—"}`);
  lines.push(`- с даты: ${filters.since || "—"}`);
  lines.push(`- по дату: ${filters.until || "—"}`);
  lines.push("");

  lines.push(`Всего событий: ${items.length}`);
  lines.push("");

  items.forEach((item, index) => {
    lines.push(
      `${index + 1}. ${auditEventLabel(item.event_type)}; время=${formatDateTime(
        item.occurred_at
      )}; описание=${auditMainDescription(item)}; ID=${compact(item.id)}`
    );
  });

  lines.push("");
  lines.push("Выбранное событие:");
  if (selected) {
    lines.push(`- событие: ${auditEventLabel(selected.event_type)}`);
    lines.push(`- время: ${formatDateTime(selected.occurred_at)}`);
    lines.push(`- описание: ${auditMainDescription(selected)}`);
    lines.push("");
    lines.push("Технические данные:");
    try {
      lines.push(JSON.stringify(selected, null, 2));
    } catch {
      lines.push(String(selected));
    }
  } else {
    lines.push("—");
  }
  lines.push("");

  return `${lines.join("\n")}`;
}

export function AuditLogPage() {
  const { token, setToken } = useAuth();

  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [selected, setSelected] = useState<AuditLogItem | null>(null);

  const [eventType, setEventType] = useState("");
  const [actorUserId, setActorUserId] = useState("");
  const [since, setSince] = useState("");
  const [until, setUntil] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [targetTypeFilter, setTargetTypeFilter] = useState("");
  const [targetIdFilter, setTargetIdFilter] = useState("");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const loadList = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const list = await api.audit.list(
        token,
        {
          event_type: eventType.trim() || undefined,
          actor_user_id: actorUserId.trim() || undefined,
          since: dateStartIso(since) || undefined,
          until: dateEndIso(until) || undefined,
        },
        ac.signal
      );
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить журнал событий");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, eventType, actorUserId, since, until, setToken]);

  const eventOptions = useMemo(() => {
    const map = new Map<string, string>();

    for (const item of items) {
      const event = String(item.event_type ?? "").trim();
      if (event) {
        map.set(event, auditEventLabel(event));
      }
    }

    return Array.from(map.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label, "ru"));
  }, [items]);

  const filteredItems = useMemo(() => {
    const query = normalizeSearch(searchQuery);
    const targetType = targetTypeFilter.trim();
    const targetId = targetIdFilter.trim().toLowerCase();

    return items.filter((item) => {
      const details = detailsOf(item);
      const itemTargetType = String(details.target_type ?? "").trim();
      const itemTargetId = String(details.target_id ?? "").trim();
      const actorUserIdValue = compact((item as Record<string, unknown>).actor_user_id);

      if (targetType && itemTargetType !== targetType) return false;
      if (targetId && !itemTargetId.toLowerCase().includes(targetId)) return false;

      if (query) {
        const text = [
          item.id,
          shortId(item.id),
          item.event_type,
          auditEventLabel(item.event_type),
          item.occurred_at,
          actorUserIdValue,
          shortId(actorUserIdValue),
          itemTargetType,
          targetTypeLabel(itemTargetType),
          itemTargetId,
          shortId(itemTargetId),
          auditMainDescription(item),
          compact(details),
        ]
          .map(normalizeSearch)
          .join(" ");

        if (!text.includes(query)) return false;
      }

      return true;
    });
  }, [items, searchQuery, targetTypeFilter, targetIdFilter]);

  const exportCsv = useCallback(() => {
    downloadCsvFile("audit-log.csv", auditCsvRows(filteredItems));
  }, [filteredItems]);

  const exportXlsx = useCallback(() => {
    downloadXlsxFile("audit-log.xlsx", auditCsvRows(filteredItems), "AuditLog");
  }, [filteredItems]);

  const exportPdf = useCallback(() => {
    downloadPdfTextFile(
      "audit-log-report.pdf",
      "Отчет по журналу аудита",
      buildAuditReportText(filteredItems, selected, {
        eventType,
        actorUserId,
        since,
        until,
      })
    );
  }, [filteredItems, selected, eventType, actorUserId, since, until]);

  const exportJson = useCallback(() => {
    downloadJsonFile("audit-log.json", filteredItems);
  }, [filteredItems]);

  const exportSelectedJson = useCallback(() => {
    if (!selected) return;
    const id = String(selected.id ?? "selected");
    downloadJsonFile(`audit-log-${id}.json`, selected);
  }, [selected]);

  useEffect(() => {
    loadList();
    return () => abortRef.current?.abort();
  }, [loadList]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 10,
            alignItems: "baseline",
            flexWrap: "wrap",
          }}
        >
          <h2 style={{ margin: 0 }}>Журнал аудита</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={loadList} disabled={loading}>
              Обновить
            </button>

            <ActionMenu
              label="Экспорт"
              items={[
                { label: "CSV", onClick: exportCsv, disabled: filteredItems.length === 0 },
                { label: "XLSX", onClick: exportXlsx, disabled: filteredItems.length === 0 },
                { label: "JSON", onClick: exportJson, disabled: filteredItems.length === 0 },
                { label: "PDF", onClick: exportPdf, disabled: filteredItems.length === 0 },
                {
                  label: "Выбранное событие",
                  onClick: exportSelectedJson,
                  disabled: !selected,
                },
              ]}
            />
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 12, ...styles.card, background: "#f9fafb" }}>
          <div style={{ fontWeight: 700, marginBottom: 10 }}>Фильтры журнала</div>

          <div style={styles.grid2}>
            <div>
              <label>Поиск</label>
              <input
                style={styles.input}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Событие, объект, ID, details"
              />
            </div>

            <div>
              <label>Тип события</label>
              <select
                style={styles.input}
                value={eventType}
                onChange={(e) => setEventType(e.target.value)}
              >
                <option value="">Все события</option>
                {eventOptions.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label>ID пользователя</label>
              <input
                style={styles.input}
                value={actorUserId}
                onChange={(e) => setActorUserId(e.target.value)}
                placeholder="UUID пользователя"
              />
            </div>

            <div>
              <label>Тип объекта</label>
              <select
                style={styles.input}
                value={targetTypeFilter}
                onChange={(e) => setTargetTypeFilter(e.target.value)}
              >
                <option value="">Все объекты</option>
                <option value="user">Пользователь</option>
                <option value="auth_session">Сессия</option>
                <option value="election">Голосование</option>
                <option value="election_invite">Приглашение</option>
                <option value="ballot">Бюллетень</option>
                <option value="experiment">Эксперимент</option>
                <option value="experiment_run">Запуск эксперимента</option>
                <option value="dataset">Набор данных</option>
                <option value="job">Задача</option>
              </select>
            </div>

            <div>
              <label>ID объекта</label>
              <input
                style={styles.input}
                value={targetIdFilter}
                onChange={(e) => setTargetIdFilter(e.target.value)}
                placeholder="UUID или короткий ID"
              />
            </div>

            <div>
              <label>С даты</label>
              <input
                style={styles.input}
                type="date"
                value={since}
                onChange={(e) => setSince(e.target.value)}
              />
            </div>

            <div>
              <label>По дату</label>
              <input
                style={styles.input}
                type="date"
                value={until}
                onChange={(e) => setUntil(e.target.value)}
              />
            </div>

            <div style={{ display: "flex", alignItems: "end" }}>
              <button
                type="button"
                style={styles.btn}
                onClick={() => {
                  setEventType("");
                  setActorUserId("");
                  setSince("");
                  setUntil("");
                  setSearchQuery("");
                  setTargetTypeFilter("");
                  setTargetIdFilter("");
                }}
              >
                Сбросить фильтры
              </button>
            </div>
          </div>

          <div style={{ marginTop: 10, ...styles.muted }}>
            Показано: {filteredItems.length} из {items.length}
          </div>
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && items.length === 0 ? <div style={styles.muted}>Список пуст</div> : null}
          {!loading && items.length > 0 && filteredItems.length === 0 ? (
            <div style={styles.muted}>По заданным фильтрам ничего не найдено</div>
          ) : null}

          {filteredItems.map((item, index) => {
            const id = String(item.id ?? `audit-${index}`);
            const event = String(item.event_type ?? "unknown");
            const occurredAt = item.occurred_at ?? "—";
            const actorUserID = compact((item as Record<string, unknown>).actor_user_id);

            return (
              <button
                key={id}
                type="button"
                style={{
                  ...styles.btn,
                  ...styles.card,
                  textAlign: "left",
                  padding: 12,
                }}
                onClick={() => setSelected(item)}
              >
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 10,
                    alignItems: "flex-start",
                    flexWrap: "wrap",
                  }}
                >
                  <div>
                    <div style={{ fontWeight: 800 }}>{auditEventLabel(event)}</div>
                    <div style={{ ...styles.muted, marginTop: 4 }}>
                      {auditMainDescription(item)}
                    </div>
                  </div>
                  <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <Badge text={formatDateTime(occurredAt)} />
                  </div>
                </div>

                <details style={{ marginTop: 10 }}>
                  <summary style={{ cursor: "pointer", ...styles.muted }}>
                    Технические сведения
                  </summary>
                  <div style={{ marginTop: 10 }}>
                    <KeyValueList
                      items={[
                        { label: "ID события", value: id },
                        { label: "Тип события", value: event },
                        { label: "Время", value: formatDateTime(occurredAt) },
                        { label: "ID пользователя", value: actorUserID || "—" },
                      ]}
                    />
                  </div>
                </details>
              </button>
            );
          })}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Выбранное событие</h3>
        {selected ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div>
              <div style={{ fontWeight: 800, fontSize: 18 }}>
                {auditEventLabel(selected.event_type)}
              </div>
              <div style={{ ...styles.muted, marginTop: 4 }}>
                {auditMainDescription(selected)}
              </div>
            </div>

            <KeyValueList
              items={[
                { label: "Время", value: formatDateTime(selected.occurred_at) },
                {
                  label: "Пользователь",
                  value: compact((selected as Record<string, unknown>).actor_user_id) || "—",
                },
                { label: "Тип события", value: auditEventLabel(selected.event_type) },
              ]}
            />

            <details>
              <summary style={{ cursor: "pointer", ...styles.muted }}>
                Технические сведения
              </summary>
              <div style={{ marginTop: 10 }}>
                <JsonBlock value={selected} />
              </div>
            </details>
          </div>
        ) : (
          <div style={styles.muted}>Ничего не выбрано</div>
        )}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug list</h3>
          <JsonBlock value={items} />
        </div>
      ) : null}
    </div>
  );
}