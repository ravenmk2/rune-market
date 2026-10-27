import { useEffect, useRef, useState } from "react";
import { useTheme } from "../context/ThemeContext";
import "./ThemeSwitch.css";

const options = [{ value: "light", label: "浅色" }, { value: "dark", label: "深色" }, { value: "system", label: "跟随系统" }] as const;

export function ThemeSwitch() {
  const { theme } = useTheme();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    const outside = (e: PointerEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [open]);
  const label = `主题：${options.find(o => o.value === theme)!.label}`;
  return <div className="theme-switch" ref={root} onBlur={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setOpen(false); }} onKeyDown={e => { if (e.key === "Escape") { setOpen(false); trigger.current?.focus(); } }}>
    <button className="theme-trigger" ref={trigger} type="button" title={label} aria-label={label} aria-expanded={open} onClick={() => setOpen(!open)}>
      <ThemeIcon theme={theme} />
    </button>
    {open && <div className="theme-options"><ThemeOptions onSelect={() => { setOpen(false); trigger.current?.focus(); }} /></div>}
  </div>;
}

export function ThemeIcon({ theme }: { theme: "light" | "dark" | "system" }) {
  return (<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        {theme === "light" ? <><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/></> : theme === "dark" ? <path d="M20.5 13A8.5 8.5 0 0 1 11 3.5 8.5 8.5 0 1 0 20.5 13Z"/> : <><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/></>}
      </svg>);
}

export function ThemeOptions({ onSelect }: { onSelect?: () => void }) {
  const { theme, setTheme } = useTheme();
  return <div className="theme-choices" role="group" aria-label="外观主题">{options.map(option => (
    <button key={option.value} type="button" aria-pressed={theme === option.value} onClick={() => { setTheme(option.value); onSelect?.(); }}>
      <ThemeIcon theme={option.value} /><span>{option.label}</span><span className="theme-check" aria-hidden="true">{theme === option.value ? "✓" : ""}</span>
    </button>
  ))}</div>;
}
