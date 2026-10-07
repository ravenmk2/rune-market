import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { skillsApi } from "../../api/skills";
import type { MySkillItem } from "../../api/skills";
import { ApiError } from "../../api/client";

export function EditSkillPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();

  const [item, setItem] = useState<MySkillItem | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [tags, setTags] = useState("");
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
        if (found) setTags(found.tags.join(", "));
      })
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "加载失败"));
    return () => {
      cancelled = true;
    };
  }, [id]);

  async function saveTags(e: FormEvent) {
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
      await skillsApi.updateTags(item.id, tagList);
      setItem({ ...item, tags: tagList });
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败,请稍后重试");
    } finally {
      setSaving(false);
    }
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
            名称与简介来自包内 SKILL.md,如需修改请
            <Link to="/publish" style={{ fontWeight: 600 }}>
              发布新版本
            </Link>
            。
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
          <h2>标签</h2>
          <form onSubmit={saveTags}>
            <div className="field">
              <input
                className="input"
                id="f-tags"
                type="text"
                aria-label="标签"
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
    </main>
  );
}
