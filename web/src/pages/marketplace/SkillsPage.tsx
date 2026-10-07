export function SkillsPage() {
  return (
    <main className="container">
      <section className="hero">
        <h1>
          智能体<em>技能</em>与<em>设计系统</em>市场
        </h1>
        <p>为智能体寻找技能,为产品定义风格。</p>
      </section>

      <nav className="tabs">
        <a className="active">Skills</a>
      </nav>

      <div
        className="panel panel-pad"
        style={{ textAlign: "center", padding: "72px 24px", margin: "28px 0 40px" }}
      >
        <p style={{ fontFamily: "var(--serif)", fontSize: 22, marginBottom: 8 }}>敬请期待</p>
        <p className="muted">制品市场正在筹备中,敬请期待。</p>
      </div>
    </main>
  );
}
