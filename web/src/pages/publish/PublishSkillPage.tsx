import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { archivesApi, skillsApi } from "../../api/skills";
import type { SkillDetail, ValidateReport } from "../../api/skills";
import { imagesApi } from "../../api/images";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { SkillIcon } from "../../components/SkillIcon";
import { Dropzone } from "../../components/Dropzone";
import { CheckList } from "../../components/CheckList";
import { ValidationSummary } from "../../components/ValidationSummary";
import { HarnessChip } from "../../components/HarnessChip";
import { PermList } from "../../components/PermList";
import { AvatarCropDialog } from "../../components/AvatarCropDialog";
import { formatBytes, shortSha } from "../../utils/format";
import { skillPublishVersionPath } from "../../utils/paths";
import { compareVersions, highestVersion, isValidVersion, suggestVersions } from "../../utils/semver";

export function PublishSkillPage() {
  const { ns = "", name = "" } = useParams();
  return <PublishSkillForm key={`${ns}/${name}`} namespace={ns} skillName={name} />;
}

function PublishSkillForm({ namespace, skillName }: { namespace: string; skillName: string }) {
  const isNewVersion = !!namespace && !!skillName;
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const iconFileRef = useRef<HTMLInputElement>(null);
  const [target, setTarget] = useState<SkillDetail | null>(null);
  const [highest, setHighest] = useState("");
  const [targetLoading, setTargetLoading] = useState(isNewVersion);
  const [targetError, setTargetError] = useState("");
  const [existingSkill, setExistingSkill] = useState<SkillDetail | null>(null);
  const uploadSequence = useRef(0);

  const [file, setFile] = useState<File | null>(null);
  const [archive, setArchive] = useState<{ sha256: string; size: number } | null>(null);
  const [report, setReport] = useState<ValidateReport | null>(null);
  const [validating, setValidating] = useState(false);
  const [validateError, setValidateError] = useState("");
  const [displayName, setDisplayName] = useState("");
  const displayNameDirty = useRef(false);
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

  useEffect(() => {
    if (!isNewVersion || authLoading) return;
    if (!user || user.username !== namespace) {
      setTargetError("只有技能作者可以发布新版本");
      setTargetLoading(false);
      return;
    }
    let cancelled = false;
    Promise.all([skillsApi.detail(namespace, skillName), skillsApi.versions(namespace, skillName)])
      .then(([skill, versions]) => {
        if (cancelled) return;
        const maximum = highestVersion(versions.map((v) => v.version));
        setTarget(skill);
        setHighest(maximum);
        setDisplayName(skill.display_name || skill.name);
        setTags(skill.tags.join(", "));
        setVersion(suggestVersions(maximum).patch);
      })
      .catch((e) => !cancelled && setTargetError(e instanceof ApiError ? e.message : "无法加载目标技能，请重试"))
      .finally(() => !cancelled && setTargetLoading(false));
    return () => { cancelled = true; };
  }, [isNewVersion, authLoading, user, namespace, skillName]);

  async function onFile(f: File) {
    if (authLoading || targetLoading || targetError || publishing) return;
    const sequence = ++uploadSequence.current;
    setFile(f);
    setReport(null);
    setExistingSkill(null);
    setArchive(null);
    setValidateError("");
    setError("");
    setValidating(true);
    try {
      const r = await archivesApi.upload(f);
      if (sequence !== uploadSequence.current) return;
      setReport(r.report);
      if (isNewVersion && r.report.metadata.name !== skillName) {
        setValidateError(`包内名称为 ${r.report.metadata.name}，需要上传 ${skillName} 的技能包`);
        return;
      }
      if (!isNewVersion && user && !r.report.checks.some((c) => c.level === "error")) {
        try {
          const existing = await skillsApi.detail(user.username, r.report.metadata.name);
          if (sequence !== uploadSequence.current) return;
          setExistingSkill(existing);
          return;
        } catch (e) {
          // Only a definite 404 means this is a new skill. A failed lookup
          // must not silently enable the legacy publish path.
          if (!(e instanceof ApiError && e.status === 404)) throw e;
        }
      }
      if (sequence !== uploadSequence.current) return;
      setArchive({ sha256: r.sha256, size: r.size });
      if (!isNewVersion && !displayNameDirty.current) setDisplayName(r.report.metadata.name);
      if (!descDirty) setDescription(r.report.metadata.description);
    } catch (e) {
      if (sequence === uploadSequence.current) setValidateError(e instanceof ApiError ? e.message : "校验请求失败,请重试");
    } finally {
      if (sequence === uploadSequence.current) setValidating(false);
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
  const reportBlocked = hasError || !!validateError || !!existingSkill;
  const canSubmit =
    !!user && !!archive && !!report && !hasError && !validateError && !existingSkill && !validating && !publishing && !iconBusy && !targetLoading && !targetError && (!isNewVersion || !!target);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit || !archive || !report) {
      setError("请先选择压缩包并通过校验");
      return;
    }
    if (!isValidVersion(version.trim())) {
      setError("版本号需为语义化版本,如 1.0.0");
      document.getElementById("f-version")?.focus();
      return;
    }
    if (isNewVersion && highest && compareVersions(version.trim(), highest) <= 0) {
      setError(`新版本必须高于当前最高版本 ${highest}`);
      document.getElementById("f-version")?.focus();
      return;
    }
    if (!displayName.trim() || Array.from(displayName.trim()).length > 64) {
      setError("展示名称需为 1–64 个字符");
      document.getElementById("f-display-name")?.focus();
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
        skill_id: target?.id,
        display_name: displayName.trim(),
        version: version.trim(),
        tags: tagList,
        description: description.trim(),
        icon: icon?.sha256,
      });
      navigate(`/s/${skill.namespace}/${skill.name}`);
    } catch (err) {
      if (err instanceof ApiError && err.code === "version_exists") {
        setError("此版本已经发布，请选择更高的版本号；已有版本不会被覆盖");
      } else {
        setError(err instanceof ApiError ? err.message : "发布失败,请稍后重试");
      }
      if (target) {
        try {
          const versions = await skillsApi.versions(namespace, skillName);
          setHighest(highestVersion(versions.map((v) => v.version)));
        } catch { /* Keep the original publish error visible. */ }
      }
    } finally {
      setPublishing(false);
    }
  }

  if (!authLoading && !user) {
    return (
      <main className="container">
        <section className="page-head">
          <h1>{isNewVersion ? "发布新版本" : "发布技能"}</h1>
          <p className="sub">{isNewVersion ? "为当前技能上传新的技能包，历史版本会保留。" : "上传技能包并填写展示信息，校验通过后发布。"}</p>
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
        <h1>{isNewVersion ? "发布新版本" : "发布技能"}</h1>
        <p className="sub">{isNewVersion ? "为当前技能上传新的技能包，历史版本会保留。" : "上传技能包并填写展示信息，校验通过后发布。"}</p>
      </section>

      {isNewVersion ? (
        <div className="panel panel-pad publish-target" style={{ marginTop: 22 }}>
          {targetLoading ? <p className="muted">加载技能信息…</p> : targetError ? (
            <div className="alert err" role="alert">{targetError}</div>
          ) : target && (
            <>
              <SkillIcon url={target.icon_url} />
              <div>
                <strong>{target.display_name || target.name}</strong>
                <div className="skill-original-name">{target.namespace}/{target.name}</div>
                <div className="hint">{highest ? `当前最高版本 v${highest}` : "尚无历史版本"} · <Link to={`/s/${namespace}/${skillName}`}>查看技能</Link></div>
              </div>
            </>
          )}
        </div>
      ) : (
        <nav className="tabs">
          <a className="active">发布技能</a>
          <Link to="/publish/design">发布 DESIGN.md</Link>
        </nav>
      )}

      <div className="layout-detail" style={{ marginTop: 26 }}>
        <div>
          <div className="panel panel-pad">
            {existingSkill && (
              <div className="alert warn" role="alert" style={{ marginBottom: 18 }}>
                <span>这个技能已经存在，请从“发布新版本”入口继续。</span>
                <Link to={skillPublishVersionPath(existingSkill.namespace, existingSkill.name)}>发布新版本 →</Link>
              </div>
            )}
            <fieldset className="publish-fields" disabled={authLoading || targetLoading || !!targetError || publishing}>
              <Dropzone
                disabled={authLoading || targetLoading || !!targetError || publishing}
                accept=".zip,.tar,.tar.gz,.tgz"
                title={
                  file
                    ? `已选择:${file.name}(${formatBytes(file.size)}),点击或拖拽可重新选择`
                    : "拖拽 zip / tar / tar.gz 到此处,或点击选择文件"
                }
                sub="根目录须包含 SKILL.md,大小上限 20 MB"
                onFile={onFile}
              />
              <ValidationSummary checks={report?.checks} busy={validating} error={validateError} blocked={!!existingSkill} detailsId="publish-checks" />
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
                  {isNewVersion && (
                    <>
                      <div className="version-options" aria-label="版本建议">
                        {([
                          ["patch", "修订"], ["minor", "次版本"], ["major", "主版本"],
                        ] as const).map(([kind, label]) => {
                          const suggestion = suggestVersions(highest)[kind];
                          return <button key={kind} type="button" className={`chip${version === suggestion ? " active" : ""}`} onClick={() => setVersion(suggestion)}>{kind === "patch" && highest.split("+")[0].includes("-") ? "正式版" : label} {suggestion}</button>;
                        })}
                      </div>
                      <div className="hint">{highest ? `v${highest}` : "首次版本"} → v{version || "…"}</div>
                    </>
                  )}
                  <div className="hint">支持语义化版本，如 1.2.4、1.3.0-beta.1；同一版本不能重复发布</div>
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

                <details className="publish-metadata" open={!isNewVersion}>
                  <summary>{isNewVersion ? "展示信息（默认沿用，可展开修改）" : "展示信息"}</summary>
                  <div className="field">
                    <label htmlFor="f-display-name">展示名称</label>
                    <input
                      className="input"
                      id="f-display-name"
                      required
                      placeholder="如：技能创建者"
                      value={displayName}
                      onChange={(e) => {
                        setDisplayName(e.target.value);
                        displayNameDirty.current = true;
                      }}
                    />
                    <div className="hint">默认与包内名称相同，可修改为便于阅读的名称（1–64 个字符）</div>
                    {report && <div className="hint">原名称：<span className="mono">{report.metadata.name}</span></div>}
                  </div>
                  <div className="field">
                    <label>图标(可选)</label>
                    <div className="icon-field">
                      <SkillIcon url={icon?.previewUrl || target?.icon_url} className="detail-icon" />
                      <div style={{ display: "flex", gap: 10 }}>
                        <button
                          className="btn btn-outline btn-sm"
                          type="button"
                          disabled={iconBusy}
                          onClick={() => iconFileRef.current?.click()}
                        >
                          {icon || target?.icon_url ? "更换图片" : "选择图片"}
                        </button>
                        {icon && (
                          <button
                            className="btn btn-outline btn-sm"
                            type="button"
                            disabled={iconBusy}
                            onClick={clearIcon}
                          >
                            {target?.icon_url ? "沿用原图标" : "移除"}
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
                    <div className="hint">PNG / JPEG，裁剪为方形后导出 256px；发布新版本默认沿用原图标</div>
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

                </details>

                {error && (
                  <div className="alert err" style={{ marginBottom: 16 }}>
                    <span>!</span>
                    <span>{error}</span>
                  </div>
                )}

                <button className="btn btn-accent" type="submit" disabled={!canSubmit}>
                  {publishing ? "发布中…" : isNewVersion ? `发布新版本 v${version}` : `发布技能 v${version}`}
                </button>
                {report && hasError && (
                  <span style={{ marginLeft: 12, fontSize: 13, color: "var(--err)" }}>
                    存在错误级检查项,无法发布
                  </span>
                )}
              </form>
            </fieldset>
          </div>

          <div className="panel panel-pad">
            <h2>上传约定</h2>
            <ul className="muted" style={{ fontSize: 14, marginLeft: 20 }}>
              <li style={{ margin: "6px 0" }}>
                支持的压缩格式:<code className="mono">.zip</code>、<code className="mono">.tar</code>、
                <code className="mono">.tar.gz</code>
              </li>
              <li style={{ margin: "6px 0" }}>SKILL.md 必须位于压缩包根目录</li>
              <li style={{ margin: "6px 0" }}>上传后会检查文件结构与必填信息，并保留扩展字段</li>
              <li style={{ margin: "6px 0" }}>制品名在你的命名空间下唯一</li>
              <li style={{ margin: "6px 0" }}>
                更新已有技能请使用“发布新版本”，历史版本会保留，可在详情页切换
              </li>
            </ul>
          </div>
        </div>

        <aside>
          <div className="panel panel-pad validation-details" id="publish-checks" tabIndex={-1}>
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
                  className={`alert ${reportBlocked ? "warn" : "ok"}`}
                  style={{ marginBottom: 14 }}
                >
                  <span>{reportBlocked ? "!" : "✓"}</span>
                  <span>
                    <b>{hasError || validateError ? "请修正校验问题后重新上传" : existingSkill ? "请使用发布新版本入口继续" : "校验通过，可发布"}</b>
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
