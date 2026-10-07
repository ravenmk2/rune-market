import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { designsApi } from "../../api/designs";
import type { DesignValidateReport } from "../../api/designs";
import { blobsApi } from "../../api/blobs";
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
  const [report, setReport] = useState<DesignValidateReport | null>(null);
  const [validating, setValidating] = useState(false);
  const [name, setName] = useState("");
  const [version, setVersion] = useState("");
  const [tags, setTags] = useState("");
  const [summary, setSummary] = useState("");
  const [desktop, setDesktop] = useState<UploadedImage | null>(null);
  const [mobile, setMobile] = useState<UploadedImage | null>(null);
  const [imageError, setImageError] = useState("");
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState("");

  async function onFile(f: File) {
    setFile(f);
    setReport(null);
    setError("");
    setValidating(true);
    try {
      setReport(await designsApi.validate(name.trim(), f));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "校验请求失败,请重试");
    } finally {
      setValidating(false);
    }
  }

  async function uploadImage(f: File, kind: "desktop" | "mobile") {
    setImageError("");
    try {
      const r = await blobsApi.upload(f);
      const uploaded = { sha256: r.sha256, previewUrl: URL.createObjectURL(f) };
      if (kind === "desktop") setDesktop(uploaded);
      else setMobile(uploaded);
    } catch (e) {
      setImageError(e instanceof ApiError ? e.message : "预览图上传失败,请重试");
    }
  }

  const hasError = !!report?.checks.some((c) => c.level === "error");
  const canSubmit = !!user && !!file && !!report && !hasError && !validating && !publishing;

  async function submit(e: FormEvent) {
    e.preventDefault();
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
    if (!file || !report) {
      setError("请先选择 .md 文件并通过校验");
      return;
    }
    setError("");
    setPublishing(true);
    try {
      const design = await designsApi.publish(file, {
        name: name.trim(),
        summary: summary.trim(),
        version: version.trim(),
        tags: tags
          .split(/[,，]/)
          .map((t) => t.trim())
          .filter(Boolean),
        preview_desktop: desktop?.sha256,
        preview_mobile: mobile?.sha256,
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
                  placeholder="claude-brand"
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
                <div className="hint">语义化版本,重新上传同版本号将覆盖</div>
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
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14 }}>
                  <ImageSlot
                    label="桌面端预览图"
                    hint="建议 1440×900"
                    uploaded={desktop}
                    onFile={(f) => uploadImage(f, "desktop")}
                  />
                  <ImageSlot
                    label="移动端预览图"
                    hint="建议 390×844"
                    uploaded={mobile}
                    onFile={(f) => uploadImage(f, "mobile")}
                  />
                </div>
                <div className="hint">
                  用于列表缩略图与详情页预览,发布后不可单独修改,随版本更新
                </div>
              </div>

              {imageError && (
                <div className="alert err" style={{ marginBottom: 16 }}>
                  <span>!</span>
                  <span>{imageError}</span>
                </div>
              )}
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
                同名制品的新版本会创建 artifact_version,可在详情页切换历史版本
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
  onFile,
}: {
  label: string;
  hint: string;
  uploaded: UploadedImage | null;
  onFile: (file: File) => void;
}) {
  if (uploaded) {
    return (
      <div>
        <div className="card-thumb">
          <img src={uploaded.previewUrl} alt={label} />
        </div>
        <div className="hint" style={{ fontSize: 12.5, color: "var(--ok)", marginTop: 5 }}>
          ✓ {label}已上传,可重新选择替换
          <span style={{ display: "block" }}>
            <ChangeImageButton onFile={onFile} />
          </span>
        </div>
      </div>
    );
  }
  return (
    <Dropzone
      accept="image/png,image/jpeg"
      title={label}
      sub={hint}
      onFile={onFile}
    />
  );
}

function ChangeImageButton({ onFile }: { onFile: (file: File) => void }) {
  return (
    <label style={{ color: "var(--accent)", cursor: "pointer" }}>
      重新选择
      <input
        type="file"
        accept="image/png,image/jpeg"
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
