import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { designsApi } from "../../api/designs";
import type { DesignListItem } from "../../api/designs";
import { useSite } from "../../context/SiteContext";
import type { ListResult } from "../../api/client";
import { ApiError } from "../../api/client";
import { DesignCard } from "../../components/DesignCard";
import { Pagination } from "../../components/Pagination";
import { useMarketFilters } from "../../hooks/useMarketFilters";

export function DesignsPage() {
  const { q, tag, official, page, input, setInput, tags, tagsError, retryTags, search: applySearch, pickTag, toggleOfficial, setPage } = useMarketFilters("designs");
  const [data, setData] = useState<ListResult<DesignListItem> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const tagline = useSite().site_tagline || "";

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    designsApi
      .list({ q: q || undefined, tag: tag || undefined, official: official || undefined, page })
      .then((r) => !cancelled && setData(r))
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试"))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [q, tag, official, page]);

  function search(e: FormEvent) {
    e.preventDefault();
    applySearch();
  }

  return (
    <main className="container">
      <section className="hero">
        <h1>
          智能体<em>技能</em>与<em>设计系统</em>商店
        </h1>
        {tagline && <p>{tagline}</p>}
        <form className="searchbar" onSubmit={search}>
          <span>⌕</span>
          <input
            aria-label="搜索设计系统"
            type="search"
            placeholder="搜索设计系统,例如 dark theme、docs、dashboard…"
            value={input}
            onChange={(e) => setInput(e.target.value)}
          />
        </form>
      </section>

      <nav className="tabs">
        <Link to="/">Skills</Link>
        <a className="active">
          DESIGN.md{data && <span className="count">{data.total}</span>}
        </a>
      </nav>

      <div className="filter-row">
        <span className="label">风格</span>
        <button className={`chip${!tag ? " active" : ""}`} onClick={() => pickTag("")}>
          全部
        </button>
        {tags.map((t) => (
          <button
            key={t}
            className={`chip${tag === t ? " active" : ""}`}
            aria-pressed={tag === t}
            onClick={() => pickTag(t)}
          >
            {t}
          </button>
        ))}
        <button
          className={`chip${official ? " active" : ""}`}
          aria-pressed={official}
          onClick={toggleOfficial}
        >
          ✦ 仅官方
        </button>
      </div>

      {tagsError && <p className="filter-notice" role="status">标签加载失败。<button type="button" className="text-button" onClick={retryTags}>重试</button></p>}

      {error && (
        <div className="alert err" style={{ marginBottom: 20 }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      )}

      {loading ? (
        <p className="muted" style={{ padding: "40px 0", textAlign: "center" }}>
          加载中…
        </p>
      ) : !error && data && data.items.length === 0 ? (
        <div
          className="panel panel-pad"
          style={{ textAlign: "center", padding: "60px 24px", marginBottom: 28 }}
        >
          <p className="muted">
            {q || tag || official ? "没有匹配的制品,换个条件试试。" : "还没有发布的制品。"}
          </p>
        </div>
      ) : (
        <section className="grid">
          {data?.items.map((d) => (
            <DesignCard key={`${d.namespace}/${d.name}`} design={d} />
          ))}
        </section>
      )}

      {data && (
        <Pagination page={data.page} pageSize={data.page_size} total={data.total} onChange={setPage} />
      )}
    </main>
  );
}
