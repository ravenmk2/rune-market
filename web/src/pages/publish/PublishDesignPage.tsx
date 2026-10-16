import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { designsApi } from "../../api/designs";
import type { DesignValidateReport, PreviewTheme, PreviewDevice } from "../../api/designs";
import { previewThemes, previewThemeLabels } from "../../utils/designPreviews";
import { imagesApi } from "../../api/images";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { Dropzone } from "../../components/Dropzone";
import { CheckList } from "../../components/CheckList";
import { formatBytes } from "../../utils/format";

const NAME_RE = /^[a-z0-9](-?[a-z0-9])*$/;
const SEMVER_RE = /^\d+\.\d+\.\d+$/;

interface UploadedImage {
  sha256: string;
  previewUrl: string;
}

export function PublishDesignPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [file, setFile] = useState<File | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [report, setReport] = useState<DesignValidateReport | null>(null);
  const [validating, setValidating] = useState(false);
  const [name, setName] = useState("");
  const [version, setVersion] = useState("0.1.0");
  const [tags, setTags] = useState("");
  const [summary, setSummary] = useState("");
  const [previewTheme, setPreviewTheme] = useState<PreviewTheme>("default");
  const [images, setImages] = useState<Record<string, UploadedImage | undefined>>({});
  const [uploading, setUploading] = useState<Record<string, boolean>>({});
  const [imageErrors, setImageErrors] = useState<Record<string, string>>({});
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState("");

  async function onFile(f: File) {
    setFile(f);
    setReport(null);
    setContent(null);
    setError("");
    setValidating(true);
    try {
      const text = await f.text();
      setContent(text);
      setReport(await designsApi.validate(name.trim(), text));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "校验请求失败,请重试");
    } finally {
      setValidating(false);
    }
  }

  async function uploadImage(f: File, theme: PreviewTheme, device: PreviewDevice) {
    const slot = `${theme}/${device}`;
    setImageErrors((prev) => ({ ...prev, [slot]: "" }));
    setUploading((prev) => ({ ...prev, [slot]: true }));
    try {
      const r = await imagesApi.upload(f);
      setImages((prev) => ({ ...prev, [slot]: { sha256: r.sha256, previewUrl: r.url } }));
    } catch (e) {
      setImageErrors((prev) => ({ ...prev, [slot]: e instanceof ApiError ? e.message : "预览图上传失败,请重试" }));
    } finally {
      setUploading((prev) => ({ ...prev, [slot]: false }));
    }
  }

  const hasError = !!report?.checks.some((c) => c.level === "error");
  const imageBusy = Object.values(uploading).some(Boolean);
  const canSubmit =
    !!user && !!file && content !== null && !!report && !hasError && !validating && !publishing && !imageBusy;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    if (!NAME_RE.test(name.trim()) || name.trim().length > 64) {
      setError("名称需为小写字母、数字或连字符,且不能以连字符开头/结尾(≤64 字符)");
      return;
    }
    if (!SEMVER_RE.test(version.trim())) {
      setError("版本号需为语义化版本,如 1.0.0");
      return;
    }
    if (!summary.trim()) {
      setError("请填写一句话简介");
      return;
    }
    if (!file || content === null || !report) {
      setError("请先选择 .md 文件并通过校验");
      return;
    }
    setError("");
    setPublishing(true);
    try {
      const design = await designsApi.publish(content, {
        name: name.trim(),
        summary: summary.trim(),
        version: version.trim(),
        tags: tags
          .split(/[,，]/)
          .map((t) => t.trim())
          .filter(Boolean),
        previews: previewThemes.flatMap((theme) =>
          (["desktop", "mobile"] as const).flatMap((device) => {
            const image = images[`${theme}/${device}`];
            return image ? [{ theme, device, sha256: image.sha256 }] : [];
          }),
        ),
      });
      navigate(`/d/${design.namespace}/${design.name}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "发布失败,请稍后重试");
    } finally {
      setPublishing(false);
    }
  }

  if (!authLoading && !user) {
    return (
      <main className="container">
        <section className="page-head">
          <h1>发布</h1>
          <p className="sub">上传后服务器将自动进行格式与安全校验,通过即公开发布。</p>
        </section>
        <div className="alert warn" style={{ marginTop: 20 }}>
          <span>!</span>
          <span>
            发布制品需要先<Link to="/login" style={{ fontWeight: 600 }}>登录</Link>。
          </span>
        </div>
      </main>
    );
  }

  return (
    <main className="container">
      <section className="page-head">
        <h1>发布</h1>
        <p className="sub">上传后服务器将自动进行格式与安全校验,通过即公开发布。</p>
      </section>

      <nav className="tabs">
        <Link to="/publish">发布 Skill</Link>
        <a className="active">发布 DESIGN.md</a>
      </nav>

      <div className="layout-detail" style={{ marginTop: 26 }}>
        <div>
          <div className="panel panel-pad">
            <Dropzone
              accept=".md,.markdown,text/markdown,text/plain"
              title={
                file
                  ? `已选择:${file.name}(${formatBytes(file.size)}),点击或拖拽可重新选择`
                  : "拖拽 DESIGN.md 到此处,或点击选择文件"
              }
              sub="单个 .md 文件,大小上限 1 MB"
              onFile={onFile}
            />

            <form style={{ marginTop: 22 }} onSubmit={submit}>
              <div className="field">
                <label htmlFor="f-name">名称</label>
                <input
                  className="input mono"
                  id="f-name"
                  type="text"
                  placeholder="acme-ui"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
                <div className="hint">
                  DESIGN.md 中不一定包含名称,需手动指定;小写字母/数字/连字符,在你的命名空间下唯一
                </div>
              </div>

              <div className="field">
                <label htmlFor="f-version">版本号</label>
                <input
                  className="input mono"
                  id="f-version"
                  type="text"
                  placeholder="1.0.0"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                />
                <div className="hint">语义化版本，同一制品的版本号不能重复</div>
              </div>

              <div className="field">
                <label htmlFor="f-tags">标签</label>
                <input
                  className="input"
                  id="f-tags"
                  type="text"
                  placeholder="用逗号分隔,如 浅色, 文档风"
                  value={tags}
                  onChange={(e) => setTags(e.target.value)}
                />
              </div>

              <div className="field">
                <label htmlFor="f-desc">简介</label>
                <textarea
                  className="textarea"
                  id="f-desc"
                  placeholder="例如:米白纸面、衬线标题、克制的强调色,适合文档站"
                  value={summary}
                  onChange={(e) => setSummary(e.target.value)}
                />
                <div className="hint">
                  DESIGN.md 无标准元数据字段,请用一句话说明这套设计系统的风格
                </div>
              </div>

              <div className="field">
                <label>预览图</label>
                <div className="preview-switch" role="group" aria-label="上传预览主题">
                  {previewThemes.map((theme) => (
                    <button key={theme} type="button" aria-pressed={previewTheme === theme}
                      className={previewTheme === theme ? "active" : ""}
                      onClick={() => setPreviewTheme(theme)}>
                      {previewThemeLabels[theme]}
                      {(["desktop", "mobile"] as const).some((d) => uploading[`${theme}/${d}`]) ? " · 上传中" :
                        (["desktop", "mobile"] as const).some((d) => imageErrors[`${theme}/${d}`]) ? " · 上传失败" :
                        ""}
                      {(["desktop", "mobile"] as const).some((d) => images[`${theme}/${d}`]) && " · 已添加"}
                    </button>
                  ))}
                </div>
                <div className="hint">
                  混合或中性配色放在默认组。各组图片均可选，标签用于描述风格，不影响预览切换。
                </div>
                <div className="preview-upload-grid">
                  {(["desktop", "mobile"] as const).map((device) => {
                    const slot = `${previewTheme}/${device}`;
                    return <div key={slot}>
                      <ImageSlot
                        label={`${previewThemeLabels[previewTheme]}${device === "desktop" ? "桌面端" : "移动端"}预览图`}
                        hint={device === "desktop" ? "建议 1440×900" : "建议 390×844"}
                        uploaded={images[slot] ?? null}
                        busy={!!uploading[slot]}
                        disabled={publishing}
                        onFile={(f) => uploadImage(f, previewTheme, device)}
                        onRemove={() => {
                          setImages((prev) => ({ ...prev, [slot]: undefined }));
                          setImageErrors((prev) => ({ ...prev, [slot]: "" }));
                        }}
                      />
                      {imageErrors[slot] && <p className="preview-upload-error" role="alert">{imageErrors[slot]}</p>}
                    </div>;
                  })}
                </div>
                <div className="hint">
                  图片随版本发布，用于列表封面和详情预览，发布后不可单独修改。每张支持 PNG/JPG，最大 5MB。
                  {imageBusy && " 正在上传图片，请稍候…"}
                </div>
              </div>

              {error && (
                <div className="alert err" style={{ marginBottom: 16 }}>
                  <span>!</span>
                  <span>{error}</span>
                </div>
              )}

              <button className="btn btn-accent" type="submit" disabled={!canSubmit}>
                {publishing ? "发布中…" : "校验并发布"}
              </button>
              {report && hasError && (
                <span style={{ marginLeft: 12, fontSize: 13, color: "var(--err)" }}>
                  存在错误级检查项,无法发布
                </span>
              )}
            </form>
          </div>

          <div className="panel panel-pad">
            <h2>上传约定</h2>
            <ul className="muted" style={{ fontSize: 14, marginLeft: 20 }}>
              <li style={{ margin: "6px 0" }}>
                仅接受单个 <code className="mono">.md</code> 文件,UTF-8 编码
              </li>
              <li style={{ margin: "6px 0" }}>制品名在你的命名空间下唯一</li>
              <li style={{ margin: "6px 0" }}>
                同名制品发布新版本后，可在详情页切换查看历史版本
              </li>
            </ul>
          </div>
        </div>

        <aside>
          <div className="panel panel-pad">
            <h2>校验结果</h2>
            {validating && <p className="muted">校验中…</p>}
            {!validating && !report && (
              <p className="muted" style={{ fontSize: 13.5 }}>
                选择 .md 文件后将自动开始校验。
              </p>
            )}
            {report && (
              <>
                <div
                  className={`alert ${hasError ? "err" : "ok"}`}
                  style={{ marginBottom: 14 }}
                >
                  <span>{hasError ? "✗" : "✓"}</span>
                  <span>
                    <b>{hasError ? "校验未通过,请修正后重新上传" : "校验通过,可发布"}</b>
                  </span>
                </div>
                <CheckList checks={report.checks} />
              </>
            )}
          </div>
        </aside>
      </div>
    </main>
  );
}

/* ---------- 预览图上传槽位 ---------- */

function ImageSlot({
  label,
  hint,
  uploaded,
  busy,
  disabled,
  onFile,
  onRemove,
}: {
  label: string;
  hint: string;
  uploaded: UploadedImage | null;
  busy: boolean;
  disabled: boolean;
  onFile: (file: File) => void;
  onRemove: () => void;
}) {
  if (uploaded) {
    return (
      <div>
        <div className="card-thumb">
          <img src={uploaded.previewUrl} alt={label} />
        </div>
        <div className="hint" style={{ fontSize: 12.5, color: "var(--ok)", marginTop: 5 }}>
          {busy ? "正在上传替换图片…" : `✓ ${label}已上传`}
          <span style={{ display: "block" }}>
            <ChangeImageButton onFile={onFile} disabled={disabled || busy} />
            <button type="button" className="btn btn-outline btn-sm" style={{ marginLeft: 10 }}
              disabled={disabled || busy} onClick={onRemove}>移除</button>
          </span>
        </div>
      </div>
    );
  }
  return (
    <Dropzone
      accept="image/png,image/jpeg"
      title={busy ? "正在上传…" : label}
      sub={hint}
      disabled={disabled || busy}
      onFile={onFile}
    />
  );
}

function ChangeImageButton({ onFile, disabled }: { onFile: (file: File) => void; disabled: boolean }) {
  return (
    <label style={{ color: "var(--accent)", cursor: disabled ? "default" : "pointer", opacity: disabled ? 0.5 : 1 }}>
      重新选择
      <input
        type="file"
        accept="image/png,image/jpeg"
        disabled={disabled}
        style={{ display: "none" }}
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) onFile(f);
          e.target.value = "";
        }}
      />
    </label>
  );
}
