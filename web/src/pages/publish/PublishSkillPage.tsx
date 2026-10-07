import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { skillsApi } from "../../api/skills";
import type { ValidateReport } from "../../api/skills";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { Dropzone } from "../../components/Dropzone";
import { CheckList } from "../../components/CheckList";
import { HarnessChip } from "../../components/HarnessChip";
import { PermList } from "../../components/PermList";
import { formatBytes, shortSha } from "../../utils/format";

const SEMVER_RE = /^\d+\.\d+\.\d+$/;

export function PublishSkillPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [file, setFile] = useState<File | null>(null);
  const [report, setReport] = useState<ValidateReport | null>(null);
  const [validating, setValidating] = useState(false);
  const [validateError, setValidateError] = useState("");
  const [version, setVersion] = useState("");
  const [tags, setTags] = useState("");
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState("");

  async function onFile(f: File) {
    setFile(f);
    setReport(null);
    setValidateError("");
    setError("");
    setValidating(true);
    try {
      setReport(await skillsApi.validate(f));
    } catch (e) {
      setValidateError(e instanceof ApiError ? e.message : "校验请求失败,请重试");
    } finally {
      setValidating(false);
    }
  }

  const hasError = !!report?.checks.some((c) => c.level === "error");
  const canSubmit =
    !!user && !!file && !!report && !hasError && !validating && !publishing;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!file || !report) {
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
      const skill = await skillsApi.publish(file, version.trim(), tagList);
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
    </main>
  );
}
