import React, { createContext, useContext, useMemo, useState } from "react";

export type Locale = "ru" | "en";

type Dictionary = Record<string, string>;

const dictionaries: Record<Locale, Dictionary> = {
  ru: {
    "nav.notifications": "Уведомления",
    "notifications.title": "Уведомления",
    "notifications.empty": "Список уведомлений пуст",
    "notifications.markAllRead": "Отметить все как прочитанные",
    "notifications.clearAll": "Очистить всё",
    "notifications.read": "Прочитать",
    "notifications.delete": "Удалить",
    "notifications.total": "Всего",
    "notifications.unread": "Непрочитано",
  },
  en: {
    "nav.notifications": "Notifications",
    "notifications.title": "Notifications",
    "notifications.empty": "No notifications yet",
    "notifications.markAllRead": "Mark all as read",
    "notifications.clearAll": "Clear all",
    "notifications.read": "Mark as read",
    "notifications.delete": "Delete",
    "notifications.total": "Total",
    "notifications.unread": "Unread",
  },
};

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

const STORAGE_KEY = "sv_locale";

function getInitialLocale(): Locale {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw === "en" ? "en" : "ru";
  } catch {
    return "ru";
  }
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => getInitialLocale());

  const setLocale = (next: Locale) => {
    setLocaleState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
    }
  };

  const value = useMemo<I18nContextValue>(() => {
    return {
      locale,
      setLocale,
      t: (key: string) => dictionaries[locale][key] ?? key,
    };
  }, [locale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used inside I18nProvider");
  return ctx;
}