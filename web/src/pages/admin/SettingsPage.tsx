import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { adminApi } from "../../api/admin";
import type { Settings } from "../../api/admin";
import { ApiError } from "../../api/client";

type RegMode = "open" | "approval" | "closed";

const REG_MODES: { key: RegMode; title: string; desc: string }[] = [
  { key: "open", title: "开放注册", desc: "任何人可注册并立即发布" },
  { key: "approval", title: "审核注册", desc: "注册后需管理员批准才能登录发布" },
  { key: "closed", title: "关闭注册", desc: "仅管理员可创建账号" },
];

function asBool(v: unknown): boolean {
  return v === true || v === "true";
}

export function SettingsPage() {
  const [loaded, setLoaded] = useState(false);
  const [siteName, setSiteName] = useState("");
  const [siteDescription, setSiteDescription] = useState("");
  const [siteTagline, setSiteTagline] = useState("");
  const [pageSize, setPageSize] = useState("20");
  const [regMode, setRegMode] = useState<RegMode>("open");
  const [uploadMaxMb, setUploadMaxMb] = useState("20");
  const [anonBrowse, setAnonBrowse] = useState(true);
  const [anonDownload, setAnonDownload] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");
  const [secretNotice, setSecretNotice] = useState("");

  useEffect(() => {
    let cancelled = false;
    adminApi
      .settings()
      .then((s) => {
        if (cancelled) return;
        setSiteName(s.site_name ?? "");
        setSiteDescription(s.site_description ?? "");
        setSiteTagline(s.site_tagline ?? "");
        setPageSize(String(s.page_size ?? 20));
        setRegMode(
          s.registration_mode === "approval" || s.registration_mode === "closed"
            ? s.registration_mode
            : "open",
        );
        setUploadMaxMb(String(s.upload_max_mb ?? 20));
        setAnonBrowse(asBool(s.anonymous_browse ?? true));
        setAnonDownload(asBool(s.anonymous_download));
        setLoaded(true);
      })
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "设置加载失败"));
    return () => {
      cancelled = true;
    };
  }, []);

  async function save(e: FormEvent) {
    e.preventDefault();
    const pageSizeNum = Number(pageSize);
    const uploadMaxNum = Number(uploadMaxMb);
    if (!Number.isInteger(pageSizeNum) || pageSizeNum < 1 || pageSizeNum > 200) {
      setError("每页制品数需为 1–200 的整数");
      return;
    }
    if (!Number.isFinite(uploadMaxNum) || uploadMaxNum < 1) {
      setError("上传大小上限需为不小于 1 的数字(MB)");
      return;
    }
    setSaving(true);
    setSaved(false);
    setError("");
    try {
      const patch: Settings = {
        site_name: siteName.trim(),
        site_description: siteDescription.trim(),
        site_tagline: siteTagline.trim(),
        page_size: pageSizeNum,
        registration_mode: regMode,
        upload_max_mb: uploadMaxNum,
        anonymous_browse: anonBrowse,
        anonymous_download: anonDownload,
      };
      await adminApi.saveSettings(patch);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败,请稍后重试");
    } finally {
      setSaving(false);
    }
  }

  async function regenerate() {
    if (
      !window.confirm(
        "重新生成主密钥会使全站会话立即失效(所有用户需重新登录)。确定继续吗?",
      )
    )
      return;
    setError("");
    setSecretNotice("");
    try {
      await adminApi.regenerateSecret();
      setSecretNotice("主密钥已重新生成,全站会话已失效。");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败,请稍后重试");
    }
  }

  if (error && !loaded) {
    return (
      <div className="alert err">
        <span>!</span>
        <span>{error}</span>
      </div>
    );
  }
  if (!loaded) return <p className="muted">加载中…</p>;

  return (
    <>
      <h1>系统设置</h1>
      <p className="sub">站点、注册与安全策略</p>

      <form onSubmit={save}>
        <section className="panel panel-pad" style={{ maxWidth: 640 }}>
          <h2>站点</h2>
          <div className="field">
            <label htmlFor="f-sitename">站点名称</label>
            <input
              className="input"
              id="f-sitename"
              type="text"
              value={siteName}
              onChange={(e) => setSiteName(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="f-sitedesc">站点描述</label>
            <input
              className="input"
              id="f-sitedesc"
              type="text"
              value={siteDescription}
              onChange={(e) => setSiteDescription(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="f-sitetagline">首页标语</label>
            <input
              className="input"
              id="f-sitetagline"
              type="text"
              placeholder="留空则市场首页不显示标语"
              value={siteTagline}
              onChange={(e) => setSiteTagline(e.target.value)}
            />
          </div>
          <div className="field" style={{ marginBottom: 0 }}>
            <label htmlFor="f-pagesize">每页制品数</label>
            <input
              className="input mono"
              id="f-pagesize"
              type="text"
              style={{ width: 120 }}
              value={pageSize}
              onChange={(e) => setPageSize(e.target.value)}
            />
          </div>
        </section>

        <section className="panel panel-pad" style={{ maxWidth: 640 }}>
          <h2>注册策略</h2>
          <div className="radio-cards">
            {REG_MODES.map((m) => (
              <label key={m.key} className={`radio-card${regMode === m.key ? " selected" : ""}`}>
                <input
                  type="radio"
                  name="reg"
                  checked={regMode === m.key}
                  onChange={() => setRegMode(m.key)}
                />
                <span>
                  <span className="rc-title">{m.title}</span>
                  <div className="rc-desc">{m.desc}</div>
                </span>
              </label>
            ))}
          </div>
        </section>

        <section className="panel panel-pad" style={{ maxWidth: 640 }}>
          <h2>安全与存储</h2>
          <div className="field">
            <label htmlFor="f-uploadmax">上传大小上限(MB)</label>
            <input
              className="input mono"
              id="f-uploadmax"
              type="text"
              style={{ width: 160 }}
              value={uploadMaxMb}
              onChange={(e) => setUploadMaxMb(e.target.value)}
            />
          </div>
          <div className="field" style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <span
              className={`toggle${anonBrowse ? " on" : ""}`}
              role="switch"
              aria-checked={anonBrowse}
              onClick={() => setAnonBrowse(!anonBrowse)}
            ></span>
            <span>允许匿名浏览</span>
          </div>
          <div className="field" style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <span
              className={`toggle${anonDownload ? " on" : ""}`}
              role="switch"
              aria-checked={anonDownload}
              onClick={() => setAnonDownload(!anonDownload)}
            ></span>
            <span>允许匿名下载</span>
          </div>
          <div className="field" style={{ marginBottom: 0 }}>
            <label>主密钥状态</label>
            <div className="meta-list">
              <div className="row">
                <span className="k">路径</span>
                <span className="v mono">/data/secret</span>
              </div>
              <div className="row">
                <span className="k">权限</span>
                <span className="v mono">0600</span>
              </div>
              <div className="row">
                <span className="k">生成方式</span>
                <span className="v">首次启动自动生成</span>
              </div>
            </div>
            <div style={{ marginTop: 12 }}>
              <button className="btn btn-danger btn-sm" type="button" onClick={regenerate}>
                重新生成(谨慎)
              </button>
            </div>
          </div>
        </section>

        {error && (
          <div className="alert err" style={{ maxWidth: 640, marginTop: 18 }}>
            <span>!</span>
            <span>{error}</span>
          </div>
        )}
        {secretNotice && (
          <div className="alert warn" style={{ maxWidth: 640, marginTop: 18 }}>
            <span>!</span>
            <span>{secretNotice}</span>
          </div>
        )}
        {saved && (
          <div className="alert ok" style={{ maxWidth: 640, marginTop: 18 }}>
            <span>✓</span>
            <span>设置已保存。</span>
          </div>
        )}

        <div style={{ marginTop: 22 }}>
          <button className="btn btn-primary" type="submit" disabled={saving}>
            {saving ? "保存中…" : "保存设置"}
          </button>
        </div>
      </form>
    </>
  );
}
