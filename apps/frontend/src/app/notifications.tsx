import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useAuth } from "./auth";
import { api } from "../shared/api/client";
import type { NotificationCreateReq, NotificationItem, NotificationKind } from "../shared/api/types";

type AddNotificationInput = {
  title: string;
  message: string;
  details?: string;
  action_label?: string;
  action_to?: string;
  kind?: NotificationKind;
};

type NotificationsContextValue = {
  items: NotificationItem[];
  unreadCount: number;
  addNotification: (input: AddNotificationInput) => void;
  markRead: (id: string) => void;
  markAllRead: () => void;
  removeNotification: (id: string) => void;
  clearAll: () => void;
};

const NotificationsContext = createContext<NotificationsContextValue | null>(null);

function newLocalNotification(input: AddNotificationInput): NotificationItem {
  const g = globalThis as any;
  const id =
    typeof g?.crypto?.randomUUID === "function"
      ? g.crypto.randomUUID()
      : `n-${Math.random().toString(16).slice(2)}-${Date.now().toString(16)}`;

  return {
    id,
    title: input.title,
    message: input.message,
    details: input.details,
    action_label: input.action_label,
    action_to: input.action_to,
    kind: input.kind ?? "info",
    created_at: new Date().toISOString(),
    read: false,
  };
}

export function NotificationsProvider({ children }: { children: React.ReactNode }) {
  const { token, authed } = useAuth();
  const [items, setItems] = useState<NotificationItem[]>([]);

  const refresh = useCallback(async () => {
    if (!token) {
      setItems([]);
      return;
    }

    try {
      const next = await api.notifications.list(token);
      setItems(next.slice(0, 100));
    } catch {
    }
  }, [token]);

  useEffect(() => {
    if (!authed || !token) {
      setItems([]);
      return;
    }
    void refresh();
  }, [authed, token, refresh]);

  const addNotification = useCallback(
    (input: AddNotificationInput) => {
      if (!token) {
        setItems((prev) => [newLocalNotification(input), ...prev].slice(0, 100));
        return;
      }

      const optimistic = newLocalNotification(input);
      setItems((prev) => [optimistic, ...prev].slice(0, 100));

      void (async () => {
        try {
          const created = await api.notifications.create(token, input as NotificationCreateReq);
          setItems((prev) => {
            const rest = prev.filter((item) => item.id !== optimistic.id);
            return [created, ...rest].slice(0, 100);
          });
        } catch {
          void refresh();
        }
      })();
    },
    [token, refresh]
  );

  const markRead = useCallback(
    (id: string) => {
      setItems((prev) => prev.map((item) => (item.id === id ? { ...item, read: true } : item)));

      if (!token) return;
      void (async () => {
        try {
          await api.notifications.markRead(token, id);
        } catch {
          void refresh();
        }
      })();
    },
    [token, refresh]
  );

  const markAllRead = useCallback(() => {
    setItems((prev) => prev.map((item) => ({ ...item, read: true })));

    if (!token) return;
    void (async () => {
      try {
        await api.notifications.markAllRead(token);
      } catch {
        void refresh();
      }
    })();
  }, [token, refresh]);

  const removeNotification = useCallback(
    (id: string) => {
      setItems((prev) => prev.filter((item) => item.id !== id));

      if (!token) return;
      void (async () => {
        try {
          await api.notifications.remove(token, id);
        } catch {
          void refresh();
        }
      })();
    },
    [token, refresh]
  );

  const clearAll = useCallback(() => {
    setItems([]);

    if (!token) return;
    void (async () => {
      try {
        await api.notifications.clearAll(token);
      } catch {
        void refresh();
      }
    })();
  }, [token, refresh]);

  const value = useMemo<NotificationsContextValue>(() => {
    const unreadCount = items.filter((item) => !item.read).length;
    return {
      items,
      unreadCount,
      addNotification,
      markRead,
      markAllRead,
      removeNotification,
      clearAll,
    };
  }, [items, addNotification, markRead, markAllRead, removeNotification, clearAll]);

  return <NotificationsContext.Provider value={value}>{children}</NotificationsContext.Provider>;
}

export function useNotifications() {
  const ctx = useContext(NotificationsContext);
  if (!ctx) {
    throw new Error("useNotifications must be used inside NotificationsProvider");
  }
  return ctx;
}