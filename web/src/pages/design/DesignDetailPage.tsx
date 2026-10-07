import { useEffect, useState } from "react";
import { Link, NavLink, Route, Routes, useParams } from "react-router-dom";
import { designsApi, designDownloadUrl } from "../../api/designs";
import type { DesignDetail, DesignVersionMeta } from "../../api/designs";
import { ApiError } from "../../api/client";
import { OfficialBadge } from "../../components/Badge";
import { Tag } from "../../components/Tag";
import { PreviewSwitch } from "../../components/PreviewSwitch";
import type { PreviewMode } from "../../components/PreviewSwitch";
import { MarkdownView } from "../../components/MarkdownView";
import { formatCount, formatDate, shortSha } from "../../utils/format";

function tabClass({ isActive }: { isActive: boolean }) {
  return isActive ? "active" : "";
}

export function DesignDetailPage() {
  const { ns = "", name = "" } = useParams();
  const [design, setDesign] = useState<DesignDetail | null>(null);
  const [versions, setVersions] = useState<DesignVersionMeta[]>([]);
  const [selectedVer, setSelectedVer] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    Promise.all([designsApi.detail(ns, name), designsApi.versions(ns, name)])
      .then(([detail, vers]) => {
        if (cancelled) return;
        setDesign(detail);
        setVersions(vers);
        setSelectedVer(detail.latest?.version || detail.latest_version);
      })
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试"))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [ns, name]);

  if (loading) {
    return (
      <main className="container">
        <p className="muted" style={{ padding: "60px 0", textAlign: "center" }}>
          加载中…
        </p>
      </main>
    );
  }

  if (error || !design) {
    return (
      <main className="container">
        <div className="alert err" style={{ margin: "40px 0" }}>
          <span>!</span>
          <span>{error || "制品不存在"}</span>
        </div>
      </main>
    );
  }

  const current = versions.find((v) => v.version === selectedVer) ?? design.latest;
  const base = `/d/${ns}/${name}`;

  return (
    <main className="container">
      <div className="crumbs">
        <Link to="/">市场</Link> / <Link to="/designs">DESIGN.md</Link> / {name}
      </div>

      <section className="detail-head">
        <div className="detail-icon">{name.charAt(0).toUpperCase()}</div>
        <div>
          <h1>
            <span className="ns">{ns} /</span> {name} {design.official && <OfficialBadge />}
          </h1>
          <p className="detail-sub">{design.summary}</p>
          <div className="detail-meta-row">
            <span>
              发布者{" "}
              <b>
                <Link to={`/u/${design.owner.username}`}>
                  {design.owner.nickname || design.owner.username}
                </Link>
              </b>
            </span>
            <span>
              下载 <b>{formatCount(design.download_count)}</b>
            </span>
            <span>
              更新于 <b>{formatDate(design.updated_at)}</b>
            </span>
          </div>
        </div>
        <div className="detail-actions">
          <select
            className="select"
            style={{ width: "auto" }}
            value={selectedVer}
            onChange={(e) => setSelectedVer(e.target.value)}
          >
            {versions.map((v) => (
              <option key={v.version} value={v.version}>
                v{v.version}
                {v.version === design.latest_version ? "(latest)" : ""}
              </option>
            ))}
          </select>
          {selectedVer && (
            <a className="btn btn-primary" href={designDownloadUrl(ns, name, selectedVer)}>
              下载 DESIGN.md
            </a>
          )}
        </div>
      </section>

      <nav className="tabs">
        <NavLink to={base} end className={tabClass}>
          预览
        </NavLink>
        <NavLink to={`${base}/content`} className={tabClass}>
          内容
        </NavLink>
      </nav>

      <div className="layout-detail" style={{ marginTop: 20 }}>
        <div>
          <Routes>
            <Route index element={<PreviewTab version={current} />} />
            <Route
              path="content"
              element={<ContentTab ns={ns} name={name} version={selectedVer} />}
            />
          </Routes>
        </div>
        <Sidebar design={design} version={current} versions={versions} ns={ns} name={name} />
      </div>
    </main>
  );
}

/* ---------- 预览 tab ---------- */

function PreviewTab({ version }: { version?: DesignVersionMeta }) {
  const [mode, setMode] = useState<PreviewMode>("desktop");

  const url = mode === "desktop" ? version?.preview_desktop_url : version?.preview_mobile_url;
  const label = mode === "desktop" ? "桌面端" : "移动端";

  return (
    <div className="panel panel-pad">
      <div className="preview-head">
        <h2>预览</h2>
        <PreviewSwitch mode={mode} onChange={setMode} />
      </div>
      {mode === "mobile" && <p className="preview-caption">移动端预览</p>}
      <div className={`preview-frame${mode === "mobile" ? " mobile" : ""}`}>
        {url ? (
          <img src={url} alt={`${label}预览`} />
        ) : (
          <p className="muted" style={{ padding: "60px 24px", textAlign: "center" }}>
            作者未上传{label}预览图。
          </p>
        )}
      </div>
    </div>
  );
}

/* ---------- 内容 tab ---------- */

function ContentTab({ ns, name, version }: { ns: string; name: string; version: string }) {
  const [content, setContent] = useState<string | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!version) return;
    let cancelled = false;
    setContent(null);
    setError("");
    designsApi
      .content(ns, name, version)
      .then((r) => !cancelled && setContent(r.content))
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "内容加载失败"));
    return () => {
      cancelled = true;
    };
  }, [ns, name, version]);

  if (error) {
    return (
      <div className="alert err">
        <span>!</span>
        <span>{error}</span>
      </div>
    );
  }
  if (content === null) {
    return (
      <div className="panel panel-pad">
        <p className="muted">加载中…</p>
      </div>
    );
  }
  return (
    <div className="panel panel-pad">
      <MarkdownView markdown={content} />
    </div>
  );
}

/* ---------- 侧栏 ---------- */

function Sidebar({
  design,
  version,
  versions,
  ns,
  name,
}: {
  design: DesignDetail;
  version?: DesignVersionMeta;
  versions: DesignVersionMeta[];
  ns: string;
  name: string;
}) {
  return (
    <aside>
      <div className="panel panel-pad">
        <h2>信息</h2>
        <div className="meta-list">
          <div className="row">
            <span className="k">名称</span>
            <span className="v mono">{design.name}</span>
          </div>
          {version && (
            <div className="row">
              <span className="k">版本</span>
              <span className="v mono">{version.version}</span>
            </div>
          )}
          <div className="row">
            <span className="k">作者</span>
            <span className="v">
              {design.owner.nickname || design.owner.username}{" "}
              <span className="muted">(@{design.owner.username})</span>
            </span>
          </div>
          {version && (
            <div className="row">
              <span className="k">SHA-256</span>
              <span className="v mono">{shortSha(version.sha256)}</span>
            </div>
          )}
          <div className="row">
            <span className="k">更新于</span>
            <span className="v">{formatDate(design.updated_at)}</span>
          </div>
        </div>
      </div>

      {versions.length > 0 && (
        <div className="panel">
          <div className="panel-pad" style={{ paddingBottom: 8 }}>
            <h2>版本</h2>
          </div>
          {versions.map((v) => (
            <div className="version-row" key={v.version}>
              <span className="v">{v.version}</span>
              {v.version === design.latest_version && <span className="latest">Latest</span>}
              <span className="date">{formatDate(v.created_at)}</span>
              <span className="spacer"></span>
              <a className="btn btn-outline btn-sm" href={designDownloadUrl(ns, name, v.version)}>
                下载
              </a>
            </div>
          ))}
        </div>
      )}

      {design.tags.length > 0 && (
        <div className="panel panel-pad">
          <h2>标签</h2>
          <div className="stack">
            {design.tags.map((t) => (
              <Tag key={t} label={t} />
            ))}
          </div>
        </div>
      )}
    </aside>
  );
}
