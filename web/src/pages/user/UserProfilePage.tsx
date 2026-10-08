import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { usersApi } from "../../api/users";
import type { UserProfile } from "../../api/users";
import { ApiError } from "../../api/client";
import { SkillCard } from "../../components/SkillCard";
import { DesignCard } from "../../components/DesignCard";

export function UserProfilePage() {
  const { username = "" } = useParams();
  const [data, setData] = useState<UserProfile | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setError("");
    usersApi
      .profile(username)
      .then((r) => !cancelled && setData(r))
      .catch((e) => {
        if (cancelled) return;
        setError(
          e instanceof ApiError && e.status === 404
            ? "用户不存在"
            : e instanceof ApiError
              ? e.message
              : "加载失败,请稍后重试",
        );
      });
    return () => {
      cancelled = true;
    };
  }, [username]);

  if (error) {
    return (
      <main className="container">
        <div className="alert err" style={{ margin: "40px 0" }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      </main>
    );
  }
  if (!data) {
    return (
      <main className="container">
        <p className="muted" style={{ padding: "60px 0", textAlign: "center" }}>
          加载中…
        </p>
      </main>
    );
  }

  const { user, skills, designs } = data;
  const displayName = user.nickname || user.username;

  return (
    <main className="container">
      <section className="detail-head">
        <div className="detail-icon">
          {user.has_avatar && user.id ? (
            <img src={`/avatars/${user.id}_128.png`} alt={displayName} />
          ) : (
            displayName.charAt(0).toUpperCase()
          )}
        </div>
        <div>
          <h1 style={{ fontFamily: "var(--serif)", fontWeight: 500 }}>
            {displayName}{" "}
            <span className="muted" style={{ fontSize: 16, fontWeight: 400 }}>
              @{user.username}
            </span>
          </h1>
          {user.bio && <p className="detail-sub">{user.bio}</p>}
          <div className="detail-meta-row">
            <span>
              加入于 <b>{user.created_at.slice(0, 7)}</b>
            </span>
            <span>
              <b>{skills.length + designs.length}</b> 个制品
            </span>
          </div>
        </div>
      </section>

      {skills.length > 0 && (
        <>
          <h2 style={{ fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, margin: "26px 0 14px" }}>
            Skills
          </h2>
          <section className="grid">
            {skills.map((s, i) => (
              <SkillCard key={`${s.namespace}/${s.name}`} skill={s} index={i} />
            ))}
          </section>
        </>
      )}

      {designs.length > 0 && (
        <>
          <h2 style={{ fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, margin: "34px 0 14px" }}>
            DESIGN.md
          </h2>
          <section className="grid">
            {designs.map((d) => (
              <DesignCard key={`${d.namespace}/${d.name}`} design={d} />
            ))}
          </section>
        </>
      )}
    </main>
  );
}
