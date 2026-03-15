import React from "react";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";

export function ProfilePage() {
  const { me, authed } = useAuth();
  const { unreadCount, items } = useNotifications();

  const lastItems = items.slice(0, 5);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>Профиль пользователя</h2>

        <div style={{ marginBottom: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          {me?.role ? <Badge text={`role: ${me.role}`} /> : null}
          <Badge text={authed ? "session: active" : "session: inactive"} />
          <Badge text={`unread notifications: ${unreadCount}`} />
        </div>

        <KeyValueList
          items={[
            { label: "User ID", value: me?.id || "—" },
            { label: "Email", value: me?.email || "—" },
            { label: "Role", value: me?.role || "—" },
          ]}
        />
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Последние уведомления</h3>

        {lastItems.length === 0 ? (
          <div style={styles.muted}>Пока уведомлений нет</div>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {lastItems.map((item) => (
              <div
                key={item.id}
                style={{
                  ...styles.card,
                  padding: 10,
                  background: item.read ? "white" : "#f9fafb",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <div>
                    <div style={{ fontWeight: 700 }}>{item.title}</div>
                    <div style={styles.muted}>{item.message}</div>
                  </div>
                  <Badge text={item.kind} />
                </div>
                <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>{item.created_at}</div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Что пока не подключено к backend</h3>
        <div style={{ ...styles.muted, display: "grid", gap: 6 }}>
          <div>• Смена пароля и редактирование профиля пока не реализованы отдельными серверными endpoint’ами.</div>
          <div>• Экран профиля уже готов как пользовательская точка входа и может быть расширен после доработки backend.</div>
        </div>
      </div>
    </div>
  );
}