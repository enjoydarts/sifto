"use client";

import { createContext, useContext, useState, type ReactNode } from "react";

type TabsContextValue = {
  activeTab: string;
  setActiveTab: (value: string) => void;
};

const TabsContext = createContext<TabsContextValue | null>(null);

function useTabs() {
  const ctx = useContext(TabsContext);
  if (!ctx) throw new Error("useTabs must be used within <Tabs>");
  return ctx;
}

export function Tabs({
  defaultValue,
  value,
  onChange,
  children,
}: {
  defaultValue: string;
  value?: string;
  onChange?: (value: string) => void;
  children: ReactNode;
}) {
  const [internal, setInternal] = useState(defaultValue);
  const activeTab = value ?? internal;
  const setActiveTab = (v: string) => {
    if (onChange) onChange(v);
    else setInternal(v);
  };
  return (
    <TabsContext.Provider value={{ activeTab, setActiveTab }}>
      {children}
    </TabsContext.Provider>
  );
}

export function TabList({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      role="tablist"
      className={`flex gap-1 overflow-x-auto border-b border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-5 pt-2 md:px-6 md:pt-3 ${className ?? ""}`}
    >
      {children}
    </div>
  );
}

export function Tab({
  value,
  children,
  className,
}: {
  value: string;
  children: ReactNode;
  className?: string;
}) {
  const { activeTab, setActiveTab } = useTabs();
  const active = activeTab === value;
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      data-state={active ? "active" : "inactive"}
      onClick={() => setActiveTab(value)}
      className={`whitespace-nowrap rounded-t-lg px-4 py-2.5 text-sm font-medium transition-colors focus-ring ${
        active
          ? "border-b-[3px] border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel-strong)] font-semibold text-[var(--color-editorial-ink)]"
          : "border-b-[3px] border-transparent text-[var(--color-editorial-ink-faint)] hover:text-[var(--color-editorial-ink-soft)]"
      } ${className ?? ""}`}
    >
      {children}
    </button>
  );
}

export function TabPanel({
  value,
  children,
  className,
}: {
  value: string;
  children: ReactNode;
  className?: string;
}) {
  const { activeTab } = useTabs();
  if (activeTab !== value) return null;
  return (
    <div role="tabpanel" className={className}>
      {children}
    </div>
  );
}
