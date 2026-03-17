import React from "react";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";

function roleLabel(role?: string) {
  if (role === "admin") return "Администратор";
  if (role === "researcher") return "Исследователь";
  if (role === "voter") return "Голосующий";
  return role || "—";
}

function kindLabel(kind?: string) {
  if (kind === "success") return "Успех";
  if (kind === "warning") return "Предупреждение";
  if (kind === "error") return "Ошибка";
  return kind || "Уведомление";
}

export function ProfilePage() {
  const { me, authed } = useAuth();
  const { unreadCount, items } = useNotifications();

  const lastItems = items.slice(0, 5);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>Профиль пользователя</h2>

        <div style={{ marginBottom: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          {me?.role ? <Badge text={`Роль: ${roleLabel(me.role)}`} /> : null}
          <Badge text={authed ? "Сессия активна" : "Сессия неактивна"} />
          <Badge text={`Непрочитанных уведомлений: ${unreadCount}`} />
        </div>

        <KeyValueList
          items={[
            { label: "ID пользователя", value: me?.id || "—" },
            { label: "Электронная почта", value: me?.email || "—" },
            { label: "Роль", value: roleLabel(me?.role) },
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
                  <Badge text={kindLabel(item.kind)} />
                </div>
                <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>{item.created_at}</div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}