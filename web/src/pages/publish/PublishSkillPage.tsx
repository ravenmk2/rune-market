import { useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { archivesApi, skillsApi } from "../../api/skills";
import type { ValidateReport } from "../../api/skills";
import { imagesApi } from "../../api/images";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { Dropzone } from "../../components/Dropzone";
import { CheckList } from "../../components/CheckList";
import { HarnessChip } from "../../components/HarnessChip";
import { PermList } from "../../components/PermList";
import { AvatarCropDialog } from "../../components/AvatarCropDialog";
import { formatBytes, shortSha } from "../../utils/format";

const SEMVER_RE = /^\d+\.\d+\.\d+$/;

export function PublishSkillPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const iconFileRef = useRef<HTMLInputElement>(null);

  const [file, setFile] = useState<File | null>(null);
  const [archive, setArchive] = useState<{ sha256: string; size: number } | null>(null);
  const [report, setReport] = useState<ValidateReport | null>(null);
  const [validating, setValidating] = useState(false);
  const [validateError, setValidateError] = useState("");
  const [version, setVersion] = useState("0.1.0");
  const [description, setDescription] = useState("");
  const [descDirty, setDescDirty] = useState(false);
  const [tags, setTags] = useState("");
  const [icon, setIcon] = useState<{ sha256: string; previewUrl: string } | null>(null);
  const [iconBusy, setIconBusy] = useState(false);
  const [iconError, setIconError] = useState("");
  const [cropSrc, setCropSrc] = useState<string | null>(null);
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState("");

  async function onFile(f: File) {
    setFile(f);
    setReport(null);
    setArchive(null);
    setValidateError("");
    setError("");
    setValidating(true);
    try {
      const r = await archivesApi.upload(f);
      setArchive({ sha256: r.sha256, size: r.size });
      setReport(r.report);
      if (!descDirty) setDescription(r.report.metadata.description);
    } catch (e) {
      setValidateError(e instanceof ApiError ? e.message : "校验请求失败,请重试");
    } finally {
      setValidating(false);
    }
  }

  function pickIcon(f: File) {
    if (f.size > 5 * 1024 * 1024) {
      setIconError("图标文件不能超过 5 MB");
      return;
    }
    setIconError("");
    setCropSrc((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      return URL.createObjectURL(f);
    });
  }

  function closeCrop() {
    if (cropSrc) URL.revokeObjectURL(cropSrc);
    setCropSrc(null);
  }

  async function uploadIcon(blob: Blob) {
    setIconBusy(true);
    setIconError("");
    try {
      const r = await imagesApi.upload(blob);
      setIcon((prev) => {
        if (prev) URL.revokeObjectURL(prev.previewUrl);
        return { sha256: r.sha256, previewUrl: URL.createObjectURL(blob) };
      });
      closeCrop();
    } catch (e) {
      setIconError(e instanceof ApiError ? e.message : "图标上传失败,请重试");
    } finally {
      setIconBusy(false);
    }
  }

  function clearIcon() {
    setIcon((prev) => {
      if (prev) URL.revokeObjectURL(prev.previewUrl);
      return null;
    });
  }

  const hasError = !!report?.checks.some((c) => c.level === "error");
  const canSubmit =
    !!user && !!archive && !!report && !hasError && !validating && !publishing;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!archive || !report) {
      setError("请先选择压缩包并通过校验");
      return;
    }
    if (!SEMVER_RE.test(version.trim())) {
      setError("版本号需为语义化版本,如 1.0.0");
      return;
    }
    setError("");
    setPublishing(true);
    try {
      const tagList = tags
        .split(/[,，]/)
        .map((t) => t.trim())
        .filter(Boolean);
      const skill = await skillsApi.publish({
        archive: archive.sha256,
        version: version.trim(),
        tags: tagList,
        description: description.trim(),
        icon: icon?.sha256,
      });
      navigate(`/s/${skill.namespace}/${skill.name}`);
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
        <a className="active">发布 Skill</a>
        <Link to="/publish/design">发布 DESIGN.md</Link>
      </nav>

      <div className="layout-detail" style={{ marginTop: 26 }}>
        <div>
          <div className="panel panel-pad">
            <Dropzone
              accept=".zip,.tar,.tar.gz,.tgz"
              title={
                file
                  ? `已选择:${file.name}(${formatBytes(file.size)}),点击或拖拽可重新选择`
                  : "拖拽 zip / tar / tar.gz 到此处,或点击选择文件"
              }
              sub="根目录须包含 SKILL.md,大小上限 20 MB"
              onFile={onFile}
            />
            <div
              className="hint"
              style={{
                fontSize: 12.5,
                color: "var(--faint)",
                marginTop: 10,
                textAlign: "center",
              }}
            >
              名称、简介、许可等元数据将自动从包内 SKILL.md 提取
            </div>

            <form style={{ marginTop: 22 }} onSubmit={submit}>
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
                <label>图标(可选)</label>
                <div className="icon-field">
                  <div className="detail-icon">
                    {icon ? (
                      <img src={icon.previewUrl} alt="图标预览" />
                    ) : (
                      (report?.metadata.name || "?").charAt(0).toUpperCase()
                    )}
                  </div>
                  <div style={{ display: "flex", gap: 10 }}>
                    <button
                      className="btn btn-outline btn-sm"
                      type="button"
                      disabled={iconBusy}
                      onClick={() => iconFileRef.current?.click()}
                    >
                      {icon ? "更换图片" : "选择图片"}
                    </button>
                    {icon && (
                      <button
                        className="btn btn-outline btn-sm"
                        type="button"
                        disabled={iconBusy}
                        onClick={clearIcon}
                      >
                        移除
                      </button>
                    )}
                  </div>
                  <input
                    ref={iconFileRef}
                    type="file"
                    accept="image/png,image/jpeg"
                    style={{ display: "none" }}
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (f) pickIcon(f);
                      e.target.value = "";
                    }}
                  />
                </div>
                {iconError && (
                  <div className="alert err" style={{ marginTop: 10 }}>
                    <span>!</span>
                    <span>{iconError}</span>
                  </div>
                )}
                <div className="hint">PNG / JPEG,裁剪为方形后导出 256px;缺省使用名称首字母占位</div>
              </div>

              <div className="field">
                <label htmlFor="f-desc">说明</label>
                <textarea
                  className="textarea"
                  id="f-desc"
                  placeholder="留空则使用包内 SKILL.md 的描述"
                  value={description}
                  onChange={(e) => {
                    setDescription(e.target.value);
                    setDescDirty(true);
                  }}
                />
                <div className="hint">
                  校验通过后已自动填入包内描述,可修改;发布后同时作为简介与当前版本说明(≤1024
                  字符)
                </div>
              </div>

              <div className="field">
                <label htmlFor="f-tags">标签</label>
                <input
                  className="input"
                  id="f-tags"
                  type="text"
                  placeholder="用逗号分隔,如 文档处理, pdf"
                  value={tags}
                  onChange={(e) => setTags(e.target.value)}
                />
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
                支持的压缩格式:<code className="mono">.zip</code>、<code className="mono">.tar</code>、
                <code className="mono">.tar.gz</code>
              </li>
              <li style={{ margin: "6px 0" }}>SKILL.md 必须位于压缩包根目录</li>
              <li style={{ margin: "6px 0" }}>规范 6 个字段做硬性校验,厂商扩展字段原样透传保留</li>
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
            {validateError && (
              <div className="alert err">
                <span>!</span>
                <span>{validateError}</span>
              </div>
            )}
            {!validating && !validateError && !report && (
              <p className="muted" style={{ fontSize: 13.5 }}>
                选择压缩包后将自动开始校验。
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

          {report && (
            <div className="panel panel-pad">
              <h2>元数据摘要</h2>
              <div className="meta-list">
                <div className="row">
                  <span className="k">名称</span>
                  <span className="v mono">{report.metadata.name}</span>
                </div>
                {report.metadata.license && (
                  <div className="row">
                    <span className="k">许可</span>
                    <span className="v">{report.metadata.license}</span>
                  </div>
                )}
                {report.metadata.author && (
                  <div className="row">
                    <span className="k">作者</span>
                    <span className="v">{report.metadata.author}</span>
                  </div>
                )}
                <div className="row">
                  <span className="k">文件数</span>
                  <span className="v">{report.metadata.file_count}</span>
                </div>
                <div className="row">
                  <span className="k">包大小</span>
                  <span className="v">{formatBytes(report.metadata.size)}</span>
                </div>
                <div className="row">
                  <span className="k">SHA-256</span>
                  <span className="v mono">{shortSha(report.metadata.sha256)}</span>
                </div>
              </div>
            </div>
          )}

          {report && report.metadata.harnesses.length > 0 && (
            <div className="panel panel-pad">
              <h2>适用 Harness</h2>
              <div className="stack">
                {report.metadata.harnesses.map((h) => (
                  <HarnessChip key={h} id={h} />
                ))}
              </div>
            </div>
          )}

          {report && report.metadata.permissions.length > 0 && (
            <div className="panel panel-pad">
              <h2>权限要求</h2>
              <PermList permissions={report.metadata.permissions} />
            </div>
          )}
        </aside>
      </div>

      {cropSrc && (
        <AvatarCropDialog
          imageSrc={cropSrc}
          busy={iconBusy}
          title="裁剪图标"
          outputSize={256}
          cropShape="rect"
          onCancel={closeCrop}
          onConfirm={(blob) => void uploadIcon(blob)}
        />
      )}
    </main>
  );
}
