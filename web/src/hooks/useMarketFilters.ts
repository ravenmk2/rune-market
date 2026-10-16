import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api } from "../api/client";

export function useMarketFilters(kind: "skills" | "designs") {
  const [params, setParams] = useSearchParams();
  const q = params.get("q") ?? "";
  const tag = params.get("tag") ?? "";
  const official = params.get("official") === "true" || params.get("official") === "1";
  const rawPage = Number(params.get("page") ?? 1);
  const page = Number.isSafeInteger(rawPage) && rawPage > 0 ? rawPage : 1;
  const [input, setInput] = useState(q);
  const [tags, setTags] = useState<string[]>([]);
  const [tagsError, setTagsError] = useState(false);
  const [retry, setRetry] = useState(0);

  useEffect(() => { setInput(q); }, [q]);
  useEffect(() => {
    let cancelled = false;
    setTagsError(false);
    api.get<{ items: string[] }>(`/${kind}/tags`)
      .then((r) => { if (!cancelled) setTags(r.items); })
      .catch(() => { if (!cancelled) setTagsError(true); });
    return () => { cancelled = true; };
  }, [kind, retry]);

  function update(values: Record<string, string | number | boolean>) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      for (const [key, value] of Object.entries(values)) {
        if (value === "" || value === false || (key === "page" && value === 1)) next.delete(key);
        else next.set(key, String(value));
      }
      return next;
    });
  }

  return {
    q, tag, official, page, input, setInput,
    tags: tag && !tags.includes(tag) ? [tag, ...tags] : tags,
    tagsError, retryTags: () => setRetry((n) => n + 1),
    search: () => update({ q: input.trim(), page: 1 }),
    pickTag: (value: string) => update({ tag: value === tag ? "" : value, page: 1 }),
    toggleOfficial: () => update({ official: !official, page: 1 }),
    setPage: (value: number) => update({ page: value }),
  };
}
