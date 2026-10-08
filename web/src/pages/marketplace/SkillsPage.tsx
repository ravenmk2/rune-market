import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { skillsApi } from "../../api/skills";
import type { SkillListItem } from "../../api/skills";
import type { ListResult } from "../../api/client";
import { ApiError } from "../../api/client";
import { SkillCard } from "../../components/SkillCard";
import { Pagination } from "../../components/Pagination";

export function SkillsPage() {
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  const [tag, setTag] = useState("");
  const [official, setOfficial] = useState(false);
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListResult<SkillListItem> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    skillsApi
      .list({ q: q || undefined, tag: tag || undefined, official: official || undefined, page })
      .then((r) => !cancelled && setData(r))
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试"))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [q, tag, official, page]);

  // 标签筛选项来自当前结果集(无独立标签接口)
  const tags = useMemo(() => {
    const set = new Set<string>();
    data?.items.forEach((s) => s.tags.forEach((t) => set.add(t)));
    return [...set];
  }, [data]);

  function search(e: FormEvent) {
    e.preventDefault();
    setPage(1);
    setQ(input.trim());
  }

  function pickTag(t: string) {
    setPage(1);
    setTag(t === tag ? "" : t);
  }

  return (
    <main className="container">
      <section className="hero">
        <h1>
          智能体<em>技能</em>与<em>设计系统</em>市场
        </h1>
        <p>为智能体寻找技能,为产品定义风格。</p>
        <form className="searchbar" onSubmit={search}>
          <span>⌕</span>
          <input
            type="text"
            placeholder="搜索技能,例如 pdf、frontend、code review…"
            value={input}
            onChange={(e) => setInput(e.target.value)}
          />
        </form>
      </section>

      <nav className="tabs">
        <a className="active">
          Skills{data && <span className="count">{data.total}</span>}
        </a>
        <Link to="/designs">DESIGN.md</Link>
      </nav>

      <div className="filter-row">
        <span className="label">标签</span>
        <button className={`chip${!tag ? " active" : ""}`} onClick={() => pickTag("")}>
          全部
        </button>
        {tags.map((t) => (
          <button
            key={t}
            className={`chip${tag === t ? " active" : ""}`}
            onClick={() => pickTag(t)}
          >
            {t}
          </button>
        ))}
        <button
          className={`chip${official ? " active" : ""}`}
          onClick={() => {
            setPage(1);
            setOfficial(!official);
          }}
        >
          ✦ 仅官方
        </button>
      </div>

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
          <p className="muted">{q || tag || official ? "没有匹配的制品,换个条件试试。" : "还没有发布的制品。"}</p>
        </div>
      ) : (
        <section className="grid">
          {data?.items.map((s, i) => (
            <SkillCard key={`${s.namespace}/${s.name}`} skill={s} index={i} />
          ))}
        </section>
      )}

      {data && (
        <Pagination page={data.page} pageSize={data.page_size} total={data.total} onChange={setPage} />
      )}
    </main>
  );
}
