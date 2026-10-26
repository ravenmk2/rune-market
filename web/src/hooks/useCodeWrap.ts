import { useState } from "react";

const KEY = "runemarket.codeWrap";

export function useCodeWrap() {
  const [wrap, setWrap] = useState(() => {
    try { return localStorage.getItem(KEY) === "true"; } catch { return false; }
  });
  function changeWrap(value: boolean) {
    setWrap(value);
    try { localStorage.setItem(KEY, String(value)); } catch { /* Keep the in-memory preference. */ }
  }
  return [wrap, changeWrap] as const;
}
