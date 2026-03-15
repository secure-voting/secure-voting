import React, { useEffect } from "react";
import { useNotifications } from "../../app/notifications";
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
  const { items, unreadCount, markRead, markAllRead, removeNotification, clearAll } = useNotifications();

  useEffect(() => {
    if (unreadCount > 0) {
      markAllRead();
    }
  }, [unreadCount, markAllRead]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Уведомления</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Badge text={`count: ${items.length}`} />
            <button style={styles.btn} onClick={markAllRead}>
              Отметить все как прочитанные
            </button>
            <button style={styles.btnDanger} onClick={clearAll}>
              Очистить всё
            </button>
          </div>
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {items.length === 0 ? (
            <div style={styles.muted}>Список уведомлений пуст</div>
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
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <div>
                    <div style={{ fontWeight: 700 }}>{item.title}</div>
                    <div style={styles.muted}>{item.message}</div>
                    <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>{item.created_at}</div>
                  </div>

                  <div style={{ display: "flex", flexDirection: "column", gap: 8, alignItems: "end" }}>
                    <Badge text={item.kind} />
                    {!item.read ? (
                      <button style={styles.btn} onClick={() => markRead(item.id)}>
                        Прочитать
                      </button>
                    ) : null}
                    <button style={styles.btnDanger} onClick={() => removeNotification(item.id)}>
                      Удалить
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