import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

export type NotificationKind = "info" | "success" | "warning" | "error";

export type NotificationItem = {
  id: string;
  title: string;
  message: string;
  details?: string;
  action_label?: string;
  action_to?: string;
  kind: NotificationKind;
  created_at: string;
  read: boolean;
};

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

const STORAGE_KEY = "sv_notifications";

const NotificationsContext = createContext<NotificationsContextValue | null>(null);

function newNotificationId() {
  const g = globalThis as any;
  if (typeof g?.crypto?.randomUUID === "function") {
    return g.crypto.randomUUID();
  }
  return `n-${Math.random().toString(16).slice(2)}-${Date.now().toString(16)}`;
}

function readInitialState(): NotificationItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as NotificationItem[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (item) =>
        item &&
        typeof item.id === "string" &&
        typeof item.title === "string" &&
        typeof item.message === "string" &&
        (item.details === undefined || typeof item.details === "string") &&
        (item.action_label === undefined || typeof item.action_label === "string") &&
        (item.action_to === undefined || typeof item.action_to === "string") &&
        typeof item.kind === "string" &&
        typeof item.created_at === "string" &&
        typeof item.read === "boolean"
    );
  } catch {
    return [];
  }
}

function writeState(items: NotificationItem[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
  } catch {
  }
}

export function NotificationsProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<NotificationItem[]>(() => readInitialState());

  const updateItems = useCallback((next: NotificationItem[] | ((prev: NotificationItem[]) => NotificationItem[])) => {
    setItems((prev) => {
      const resolved = typeof next === "function" ? next(prev) : next;
      const trimmed = resolved.slice(0, 100);
      writeState(trimmed);
      return trimmed;
    });
  }, []);

  const addNotification = useCallback(
    (input: AddNotificationInput) => {
      const nextItem: NotificationItem = {
        id: newNotificationId(),
        title: input.title,
        message: input.message,
        details: input.details,
        action_label: input.action_label,
        action_to: input.action_to,
        kind: input.kind ?? "info",
        created_at: new Date().toISOString(),
        read: false,
      };

      updateItems((prev) => [nextItem, ...prev]);
    },
    [updateItems]
  );

  const markRead = useCallback(
    (id: string) => {
      updateItems((prev) =>
        prev.map((item) => (item.id === id ? { ...item, read: true } : item))
      );
    },
    [updateItems]
  );

  const markAllRead = useCallback(() => {
    updateItems((prev) => prev.map((item) => ({ ...item, read: true })));
  }, [updateItems]);

  const removeNotification = useCallback(
    (id: string) => {
      updateItems((prev) => prev.filter((item) => item.id !== id));
    },
    [updateItems]
  );

  const clearAll = useCallback(() => {
    updateItems([]);
  }, [updateItems]);

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