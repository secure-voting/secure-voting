import React, { useState } from "react";
import { Link } from "react-router-dom";
import { useNotifications } from "../../app/notifications";
import { useI18n } from "../../app/i18n";
import { Badge } from "../../shared/ui/Badge";
import { styles } from "../../shared/ui/styles";

function kindStyle(kind: string): React.CSSProperties {
  if (kind === "success") {
    return { borderColor: "#bbf7d0", background: "#f0fdf4" };
  }
  if (kind === "warning") {
    return { borderColor: "#fde68a", background: "#fffbeb" };
  }
  if (kind === "error") {
    return { borderColor: "#fecaca", background: "#fff1f2" };
  }
  return { borderColor: "#e5e7eb", background: "white" };
}

export function NotificationsPage() {
  const { t } = useI18n();
  const { items, unreadCount, markRead, markAllRead, removeNotification, clearAll } = useNotifications();
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

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
          <h2 style={{ margin: 0 }}>{t("notifications.title")}</h2>

          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Badge text={`${t("notifications.total")}: ${items.length}`} />
            <Badge text={`${t("notifications.unread")}: ${unreadCount}`} />

            <button
              style={styles.btn}
              onClick={markAllRead}
              disabled={unreadCount === 0}
            >
              {t("notifications.markAllRead")}
            </button>

            <button
              style={styles.btnDanger}
              onClick={clearAll}
              disabled={items.length === 0}
            >
              {t("notifications.clearAll")}
            </button>
          </div>
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {items.length === 0 ? (
            <div style={styles.muted}>{t("notifications.empty")}</div>
          ) : (
            items.map((item) => (
              <div
                key={item.id}
                style={{
                  ...styles.card,
                  ...kindStyle(item.kind),
                  padding: 12,
                  opacity: item.read ? 0.9 : 1,
                }}
              >
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 10,
                    flexWrap: "wrap",
                  }}
                >
                  <div style={{ minWidth: 260, flex: 1 }}>
                    <div style={{ fontWeight: 700 }}>{item.title}</div>
                    <div style={styles.muted}>{item.message}</div>

                    {item.details ? (
                      <div style={{ marginTop: 8 }}>
                        <button
                          style={styles.btn}
                          onClick={() =>
                            setExpanded((prev) => ({ ...prev, [item.id]: !prev[item.id] }))
                          }
                        >
                          {expanded[item.id] ? "Скрыть детали" : "Показать детали"}
                        </button>

                        {expanded[item.id] ? (
                          <div style={{ marginTop: 8, whiteSpace: "pre-wrap" }}>
                            {item.details}
                          </div>
                        ) : null}
                      </div>
                    ) : null}

                    <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                      {item.created_at}
                    </div>
                  </div>

                  <div
                    style={{
                      display: "flex",
                      flexDirection: "column",
                      gap: 8,
                      alignItems: "end",
                    }}
                  >
                    <Badge text={item.kind} />

                    {!item.read ? (
                      <button style={styles.btn} onClick={() => markRead(item.id)}>
                        {t("notifications.read")}
                      </button>
                    ) : null}

                    {item.action_label && item.action_to ? (
                      <Link to={item.action_to} style={{ textDecoration: "none" }}>
                        <button style={styles.btn}>{item.action_label}</button>
                      </Link>
                    ) : null}

                    <button style={styles.btnDanger} onClick={() => removeNotification(item.id)}>
                      {t("notifications.delete")}
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}