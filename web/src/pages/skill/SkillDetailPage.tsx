import { useEffect, useState } from "react";
import { Link, NavLink, Route, Routes, useParams } from "react-router-dom";
import { skillsApi, skillDownloadUrl } from "../../api/skills";
import type { FileContent, FileEntry, SkillDetail, VersionMeta } from "../../api/skills";
import { ApiError } from "../../api/client";
import { OfficialBadge } from "../../components/Badge";
import { Tag } from "../../components/Tag";
import { HarnessChip } from "../../components/HarnessChip";
import { PermList } from "../../components/PermList";
import { FileTree } from "../../components/FileTree";
import { CodeView } from "../../components/CodeView";
import { formatBytes, formatCount, formatDate, shortSha } from "../../utils/format";

function tabClass({ isActive }: { isActive: boolean }) {
  return isActive ? "active" : "";
}

export function SkillDetailPage() {
  const { ns = "", name = "" } = useParams();
  const [skill, setSkill] = useState<SkillDetail | null>(null);
  const [versions, setVersions] = useState<VersionMeta[]>([]);
  const [selectedVer, setSelectedVer] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    Promise.all([skillsApi.detail(ns, name), skillsApi.versions(ns, name)])
      .then(([detail, vers]) => {
        if (cancelled) return;
        setSkill(detail);
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

  if (error || !skill) {
    return (
      <main className="container">
        <div className="alert err" style={{ margin: "40px 0" }}>
          <span>!</span>
          <span>{error || "制品不存在"}</span>
        </div>
      </main>
    );
  }

  const current = versions.find((v) => v.version === selectedVer) ?? skill.latest;
  const ext = current?.filename?.includes(".")
    ? current.filename.slice(current.filename.lastIndexOf("."))
    : ".zip";
  const base = `/s/${ns}/${name}`;

  return (
    <main className="container">
      <div className="crumbs">
        <Link to="/">市场</Link> / <Link to="/">Skills</Link> / {name}
      </div>

      <section className="detail-head">
        <div className="detail-icon">{name.charAt(0).toUpperCase()}</div>
        <div>
          <h1>
            <span className="ns">{ns} /</span> {name} {skill.official && <OfficialBadge />}
          </h1>
          <p className="detail-sub">{skill.summary || current?.description}</p>
          <div className="detail-meta-row">
            <span>
              发布者{" "}
              <b>
                <Link to={`/u/${skill.owner.username}`}>
                  {skill.owner.nickname || skill.owner.username}
                </Link>
              </b>
            </span>
            {current?.license && (
              <span>
                许可 <b>{current.license}</b>
              </span>
            )}
            <span>
              下载 <b>{formatCount(skill.download_count)}</b>
            </span>
            <span>
              更新于 <b>{formatDate(skill.updated_at)}</b>
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
                {v.version === skill.latest_version ? "(latest)" : ""}
              </option>
            ))}
          </select>
          {selectedVer && (
            <a className="btn btn-primary" href={skillDownloadUrl(ns, name, selectedVer)}>
              下载 {ext}
            </a>
          )}
        </div>
      </section>

      <nav className="tabs">
        <NavLink to={base} end className={tabClass}>
          说明
        </NavLink>
        <NavLink to={`${base}/files`} className={tabClass}>
          文件{current && <span className="count">{current.file_count}</span>}
        </NavLink>
        <NavLink to={`${base}/versions`} className={tabClass}>
          版本<span className="count">{versions.length}</span>
        </NavLink>
      </nav>

      <div className="layout-detail" style={{ marginTop: 20 }}>
        <div>
          <Routes>
            <Route index element={<AboutTab version={current} />} />
            <Route path="files" element={<FilesTab ns={ns} name={name} version={selectedVer} />} />
            <Route
              path="versions"
              element={
                <VersionsTab
                  ns={ns}
                  name={name}
                  versions={versions}
                  latest={skill.latest_version}
                />
              }
            />
          </Routes>
        </div>
        <Sidebar skill={skill} version={current} />
      </div>
    </main>
  );
}

/* ---------- 说明 tab:description 文本段落(M2 不做 markdown 渲染) ---------- */

function AboutTab({ version }: { version?: VersionMeta }) {
  const text = version?.description?.trim();
  return (
    <div className="panel panel-pad">
      {text ? (
        text.split(/\n{2,}/).map((para, i) => (
          <p key={i} style={{ margin: "10px 0", color: "#3B3A35", whiteSpace: "pre-wrap" }}>
            {para}
          </p>
        ))
      ) : (
        <p className="muted">作者未提供说明。</p>
      )}
    </div>
  );
}

/* ---------- 文件 tab:目录树 + 文本预览 ---------- */

function FilesTab({ ns, name, version }: { ns: string; name: string; version: string }) {
  const [files, setFiles] = useState<FileEntry[] | null>(null);
  const [active, setActive] = useState<string | null>(null);
  const [content, setContent] = useState<FileContent | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!version) return;
    let cancelled = false;
    setFiles(null);
    setActive(null);
    setContent(null);
    skillsApi
      .files(ns, name, version)
      .then((list) => {
        if (cancelled) return;
        setFiles(list);
        const def = list.find((f) => f.path.endsWith("SKILL.md")) ?? list[0];
        if (def) setActive(def.path);
      })
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "目录树加载失败"));
    return () => {
      cancelled = true;
    };
  }, [ns, name, version]);

  useEffect(() => {
    if (!active || !version) return;
    let cancelled = false;
    setContent(null);
    skillsApi
      .file(ns, name, version, active)
      .then((c) => !cancelled && setContent(c))
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "文件加载失败"));
    return () => {
      cancelled = true;
    };
  }, [ns, name, version, active]);

  if (error) {
    return (
      <div className="alert err">
        <span>!</span>
        <span>{error}</span>
      </div>
    );
  }
  if (!files) {
    return (
      <div className="panel panel-pad">
        <p className="muted">加载中…</p>
      </div>
    );
  }
  if (files.length === 0) {
    return (
      <div className="panel panel-pad">
        <p className="muted">包内没有文件。</p>
      </div>
    );
  }

  return (
    <div className="panel">
      <div className="fileview">
        <FileTree paths={files.map((f) => f.path)} active={active} onSelect={setActive} />
        <div className="file-preview">
          <div className="file-preview-head">
            <span>{active}</span>
            {content && (
              <span className="size">
                {formatBytes(content.size)} · {content.content_type === "text" ? "text" : "binary"}
              </span>
            )}
          </div>
          {content ? (
            content.content_type === "text" ? (
              <CodeView content={content.content} />
            ) : (
              <div className="codebox muted">二进制文件,无法预览。</div>
            )
          ) : (
            <div className="codebox muted">加载中…</div>
          )}
        </div>
      </div>
    </div>
  );
}

/* ---------- 版本 tab ---------- */

function VersionsTab({
  ns,
  name,
  versions,
  latest,
}: {
  ns: string;
  name: string;
  versions: VersionMeta[];
  latest: string;
}) {
  if (versions.length === 0) {
    return (
      <div className="panel panel-pad">
        <p className="muted">暂无版本。</p>
      </div>
    );
  }
  return (
    <div className="panel">
      {versions.map((v) => (
        <div className="version-row" key={v.version}>
          <span className="v">{v.version}</span>
          {v.version === latest && <span className="latest">Latest</span>}
          <span className="date">
            {formatDate(v.created_at)} · <span className="mono">{shortSha(v.sha256)}</span> ·{" "}
            {formatBytes(v.size)}
          </span>
          <span className="spacer"></span>
          <a className="btn btn-outline btn-sm" href={skillDownloadUrl(ns, name, v.version)}>
            下载
          </a>
        </div>
      ))}
    </div>
  );
}

/* ---------- 侧栏 ---------- */

function Sidebar({ skill, version }: { skill: SkillDetail; version?: VersionMeta }) {
  return (
    <aside>
      <div className="panel panel-pad">
        <h2>信息</h2>
        <div className="meta-list">
          <div className="row">
            <span className="k">名称</span>
            <span className="v mono">{skill.name}</span>
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
              <Link to={`/u/${skill.owner.username}`}>
                {version?.author || skill.owner.nickname || skill.owner.username}
              </Link>{" "}
              <span className="muted">(@{skill.owner.username})</span>
            </span>
          </div>
          {version?.license && (
            <div className="row">
              <span className="k">许可</span>
              <span className="v">{version.license}</span>
            </div>
          )}
          {version && (
            <div className="row">
              <span className="k">SHA-256</span>
              <span className="v mono">{shortSha(version.sha256)}</span>
            </div>
          )}
          {version && (
            <div className="row">
              <span className="k">包大小</span>
              <span className="v">{formatBytes(version.size)}</span>
            </div>
          )}
        </div>
      </div>

      <div className="panel panel-pad">
        <h2>适用 Harness</h2>
        {version && version.harnesses.length > 0 ? (
          <div className="stack">
            {version.harnesses.map((h) => (
              <HarnessChip key={h} id={h} />
            ))}
          </div>
        ) : (
          <p className="muted" style={{ fontSize: 13.5 }}>
            通用(适配所有 harness)
          </p>
        )}
      </div>

      <div className="panel panel-pad">
        <h2>权限要求</h2>
        <PermList permissions={version?.permissions ?? []} />
      </div>

      {version?.compatibility && (
        <div className="panel panel-pad">
          <h2>环境要求</h2>
          <p className="muted" style={{ fontSize: 13.5 }}>
            {version.compatibility}
          </p>
        </div>
      )}

      {skill.tags.length > 0 && (
        <div className="panel panel-pad">
          <h2>标签</h2>
          <div className="stack">
            {skill.tags.map((t) => (
              <Tag key={t} label={t} />
            ))}
          </div>
        </div>
      )}
    </aside>
  );
}
