import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { skillsApi } from "../../api/skills";
import type { MySkillItem } from "../../api/skills";
import { imagesApi } from "../../api/images";
import { ApiError } from "../../api/client";
import { AvatarCropDialog } from "../../components/AvatarCropDialog";

export function EditSkillPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const iconFileRef = useRef<HTMLInputElement>(null);

  const [item, setItem] = useState<MySkillItem | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState("");
  const [icon, setIcon] = useState<{ sha256: string; url: string } | null>(null);
  const [iconRemoved, setIconRemoved] = useState(false);
  const [iconBusy, setIconBusy] = useState(false);
  const [cropSrc, setCropSrc] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");
  const [acting, setActing] = useState(false);

  useEffect(() => {
    let cancelled = false;
    skillsApi
      .mine()
      .then((r) => {
        if (cancelled) return;
        const found = r.items.find((s) => s.id === id) ?? null;
        setItem(found);
        setNotFound(!found);
        if (found) {
          setTags(found.tags.join(", "));
          setSummary(found.summary);
          // 说明是版本级字段,从详情取当前最新版本预填
          skillsApi
            .detail(found.namespace, found.name)
            .then((d) => !cancelled && setDescription(d.latest?.description ?? ""))
            .catch(() => undefined);
        }
      })
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "加载失败"));
    return () => {
      cancelled = true;
    };
  }, [id]);

  async function saveMeta(e: FormEvent) {
    e.preventDefault();
    if (!item) return;
    setSaving(true);
    setSaved(false);
    setError("");
    try {
      const tagList = tags
        .split(/[,，]/)
        .map((t) => t.trim())
        .filter(Boolean);
      const iconParam = icon ? icon.sha256 : iconRemoved ? "" : undefined;
      await skillsApi.update(item.id, {
        tags: tagList,
        summary: summary.trim(),
        description: description.trim(),
        icon: iconParam,
      });
      setItem({
        ...item,
        tags: tagList,
        summary: summary.trim(),
        icon_url: icon ? icon.url : iconRemoved ? null : item.icon_url,
      });
      setIcon(null);
      setIconRemoved(false);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败,请稍后重试");
    } finally {
      setSaving(false);
    }
  }

  function pickIcon(f: File) {
    if (f.size > 5 * 1024 * 1024) {
      setError("图标文件不能超过 5 MB");
      return;
    }
    setError("");
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
    setError("");
    try {
      const r = await imagesApi.upload(blob);
      setIcon({ sha256: r.sha256, url: r.url });
      setIconRemoved(false);
      closeCrop();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "图标上传失败,请重试");
    } finally {
      setIconBusy(false);
    }
  }

  function clearIcon() {
    setIcon(null);
    setIconRemoved(true);
  }

  async function toggleTakedown() {
    if (!item) return;
    setActing(true);
    setError("");
    try {
      if (item.status === "taken_down") await skillsApi.restore(item.id);
      else await skillsApi.takedown(item.id);
      setItem({ ...item, status: item.status === "taken_down" ? "published" : "taken_down" });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败,请稍后重试");
    } finally {
      setActing(false);
    }
  }

  async function remove() {
    if (!item) return;
    if (!window.confirm(`确定删除 ${item.name} 吗?将删除所有版本与文件,不可恢复。`)) return;
    setActing(true);
    setError("");
    try {
      await skillsApi.remove(item.id);
      navigate("/mine/skills");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败,请稍后重试");
      setActing(false);
    }
  }

  if (error && !item) {
    return (
      <main className="container">
        <div className="alert err" style={{ margin: "40px 0" }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      </main>
    );
  }
  if (notFound) {
    return (
      <main className="container">
        <div className="alert err" style={{ margin: "40px 0" }}>
          <span>!</span>
          <span>制品不存在或不属于你。</span>
        </div>
      </main>
    );
  }
  if (!item) {
    return (
      <main className="container">
        <p className="muted" style={{ padding: "60px 0", textAlign: "center" }}>
          加载中…
        </p>
      </main>
    );
  }

  const takenDown = item.status === "taken_down";
  const iconUrl = icon ? icon.url : iconRemoved ? null : item.icon_url;

  return (
    <main className="container">
      <div className="crumbs">
        <Link to="/mine/skills">我的发布</Link> /{" "}
        <Link to={`/s/${item.namespace}/${item.name}`}>{item.name}</Link> / 编辑
      </div>

      <section className="page-head" style={{ paddingTop: 18 }}>
        <h1>
          编辑{" "}
          <span className="mono" style={{ fontSize: "0.8em" }}>
            {item.name}
          </span>
        </h1>
      </section>

      <div style={{ maxWidth: 640, paddingBottom: 20 }}>
        <div className="alert ok" style={{ margin: "14px 0 18px" }}>
          <span>✓</span>
          <span>
            名称来自包内 SKILL.md,如需改名请
            <Link to="/publish" style={{ fontWeight: 600 }}>
              发布新版本
            </Link>
            ;简介与说明可直接在下方修改。
          </span>
        </div>

        {error && (
          <div className="alert err" style={{ marginBottom: 18 }}>
            <span>!</span>
            <span>{error}</span>
          </div>
        )}
        {saved && (
          <div className="alert ok" style={{ marginBottom: 18 }}>
            <span>✓</span>
            <span>已保存。</span>
          </div>
        )}

        <div className="panel panel-pad">
          <h2>基本信息</h2>
          <form onSubmit={saveMeta}>
            <div className="field">
              <label>图标</label>
              <div className="icon-field">
                <div className="detail-icon">
                  {iconUrl ? (
                    <img src={iconUrl} alt="图标" />
                  ) : (
                    item.name.charAt(0).toUpperCase()
                  )}
                </div>
                <div style={{ display: "flex", gap: 10 }}>
                  <button
                    className="btn btn-outline btn-sm"
                    type="button"
                    disabled={iconBusy}
                    onClick={() => iconFileRef.current?.click()}
                  >
                    {iconUrl ? "更换图标" : "选择图片"}
                  </button>
                  {iconUrl && (
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
              <div className="hint">PNG / JPEG,裁剪为方形后导出 256px;随"保存"一起生效</div>
            </div>
            <div className="field">
              <label htmlFor="f-summary">简介</label>
              <input
                className="input"
                id="f-summary"
                type="text"
                placeholder="一句话简介"
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
              />
              <div className="hint">展示在列表卡片与详情页头部</div>
            </div>
            <div className="field">
              <label htmlFor="f-desc">说明</label>
              <textarea
                className="textarea"
                id="f-desc"
                placeholder="详细介绍这个技能的用法与注意事项"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
              <div className="hint">作用于当前最新版本,展示在详情页"说明"标签页</div>
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
              <div className="hint">用逗号分隔</div>
            </div>
            <button className="btn btn-primary" type="submit" disabled={saving}>
              {saving ? "保存中…" : "保存"}
            </button>
          </form>
        </div>

        <div className="panel panel-pad danger">
          <h2>危险区</h2>
          <div className="danger-row">
            <div>
              <div className="dr-title">{takenDown ? "恢复制品" : "下架制品"}</div>
              <div className="dr-desc">
                {takenDown ? "恢复后重新在市场展示" : "下架后不在市场展示,已下载用户不受影响"}
              </div>
            </div>
            <button
              className="btn btn-outline btn-sm"
              type="button"
              disabled={acting}
              onClick={toggleTakedown}
            >
              {takenDown ? "恢复" : "下架"}
            </button>
          </div>
          <div className="danger-row">
            <div>
              <div className="dr-title">删除制品</div>
              <div className="dr-desc">删除所有版本与文件,不可恢复</div>
            </div>
            <button
              className="btn btn-danger btn-sm"
              type="button"
              disabled={acting}
              onClick={remove}
            >
              删除
            </button>
          </div>
        </div>
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
