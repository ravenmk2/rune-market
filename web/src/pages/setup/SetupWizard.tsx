import { useState } from "react";
import type { FormEvent } from "react";
import { ApiError } from "../../api/client";
import { setupApi } from "../../api/setup";
import type { DatabaseConfig, MysqlDatabaseConfig } from "../../api/setup";

/** 用户名规则(docs/design.md §7) */
const USERNAME_RE = /^[a-z0-9](-?[a-z0-9])*$/;

const STEP_LABELS = ["1 · 选择数据库", "2 · 初始管理员", "3 · 完成"];

function Steps({ current }: { current: number }) {
  return (
    <div className="steps">
      {STEP_LABELS.map((label, i) => {
        const n = i + 1;
        const cls = n < current ? "step done" : n === current ? "step active" : "step";
        return (
          <div key={label} className={cls}>
            {label}
          </div>
        );
      })}
    </div>
  );
}

function ErrorAlert({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="alert err" style={{ marginBottom: 18 }}>
      <span>!</span>
      <span>{message}</span>
    </div>
  );
}

/** 安装向导(三步,仅 setup 模式可达;契约 docs/design.md §8.1) */
export function SetupWizard({ initialStep = 1 }: { initialStep?: number }) {
  const [step, setStep] = useState(initialStep);
  const [dbConfig, setDbConfig] = useState<DatabaseConfig>({ driver: "sqlite", data_dir: "/data" });
  const [admin, setAdmin] = useState({ username: "", nickname: "", password: "" });

  return (
    <main className="setup-wrap">
      <div className="setup-brand">
        <span className="brand-mark">ᚱ</span>RuneMarket
      </div>

      {step === 1 && (
        <DatabaseStep
          config={dbConfig}
          onChange={setDbConfig}
          onNext={() => setStep(2)}
        />
      )}
      {step === 2 && (
        <AdminStep
          onBack={() => setStep(1)}
          onDone={(a) => {
            setAdmin(a);
            setStep(3);
          }}
        />
      )}
      {step === 3 && <DoneStep dbConfig={dbConfig} admin={admin} />}
    </main>
  );
}

/* ---------- 第一步:选择数据库 ---------- */

type TestState =
  | { kind: "idle" }
  | { kind: "testing" }
  | { kind: "ok"; message: string }
  | { kind: "err"; message: string };

function DatabaseStep({
  config,
  onChange,
  onNext,
}: {
  config: DatabaseConfig;
  onChange: (c: DatabaseConfig) => void;
  onNext: () => void;
}) {
  const [test, setTest] = useState<TestState>({ kind: "idle" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const mysql: MysqlDatabaseConfig =
    config.driver === "mysql"
      ? config
      : { driver: "mysql", host: "localhost:3306", database: "runemarket", username: "", password: "" };

  function selectSqlite() {
    onChange({ driver: "sqlite", data_dir: config.driver === "sqlite" ? config.data_dir : "/data" });
  }

  function selectMysql() {
    onChange(mysql);
    setTest({ kind: "idle" });
  }

  function patchMysql(patch: Partial<MysqlDatabaseConfig>) {
    onChange({ ...mysql, ...patch });
    setTest({ kind: "idle" });
  }

  async function testConnection() {
    setTest({ kind: "testing" });
    setError("");
    try {
      const result = await setupApi.testDatabase(mysql);
      if (result.ok === false) {
        setTest({ kind: "err", message: "连接失败" });
      } else {
        setTest({ kind: "ok", message: "连接成功" });
      }
    } catch (err) {
      setTest({ kind: "err", message: err instanceof ApiError ? err.message : "连接测试失败" });
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (config.driver === "sqlite" && !config.data_dir.trim()) {
      setError("请输入数据目录");
      return;
    }
    if (config.driver === "mysql" && (!config.host.trim() || !config.database.trim() || !config.username.trim())) {
      setError("请填写 MySQL 主机、数据库名与用户名");
      return;
    }
    setError("");
    setSaving(true);
    try {
      await setupApi.saveDatabase(config);
      onNext();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存数据库配置失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="setup-card">
      <Steps current={1} />

      <h1>选择数据库</h1>
      <p className="sub">RuneMarket 将业务数据存储在你选择的数据库中,配置写入 /data/config.toml。</p>

      <ErrorAlert message={error} />

      <form onSubmit={submit}>
        <div className="radio-cards">
          <label className={`radio-card${config.driver === "sqlite" ? " selected" : ""}`}>
            <input
              type="radio"
              name="db"
              checked={config.driver === "sqlite"}
              onChange={selectSqlite}
            />
            <div style={{ flex: 1 }}>
              <div className="rc-title">SQLite</div>
              <div className="rc-desc">单文件嵌入式数据库,零依赖,适合个人与小团队部署。</div>
              {config.driver === "sqlite" && (
                <div className="field" style={{ margin: "12px 0 0" }}>
                  <label htmlFor="f-datadir">数据目录</label>
                  <input
                    className="input mono"
                    id="f-datadir"
                    type="text"
                    value={config.data_dir}
                    onChange={(e) => onChange({ driver: "sqlite", data_dir: e.target.value })}
                  />
                  <div className="hint">
                    数据库文件将保存为 {(config.data_dir || "/data").replace(/\/+$/, "")}/runemarket.db
                  </div>
                </div>
              )}
            </div>
          </label>

          <label className={`radio-card${config.driver === "mysql" ? " selected" : ""}`}>
            <input
              type="radio"
              name="db"
              checked={config.driver === "mysql"}
              onChange={selectMysql}
            />
            <div style={{ flex: 1 }}>
              <div className="rc-title">MySQL</div>
              <div className="rc-desc">适合多实例部署或已有 MySQL 基础设施。</div>
              {config.driver === "mysql" && (
                <>
                  <div className="field-2col" style={{ marginTop: 12 }}>
                    <div className="field" style={{ marginBottom: 12 }}>
                      <label htmlFor="f-host">主机</label>
                      <input
                        className="input mono"
                        id="f-host"
                        type="text"
                        value={mysql.host}
                        onChange={(e) => patchMysql({ host: e.target.value })}
                      />
                    </div>
                    <div className="field" style={{ marginBottom: 12 }}>
                      <label htmlFor="f-dbname">数据库名</label>
                      <input
                        className="input mono"
                        id="f-dbname"
                        type="text"
                        value={mysql.database}
                        onChange={(e) => patchMysql({ database: e.target.value })}
                      />
                    </div>
                    <div className="field" style={{ marginBottom: 0 }}>
                      <label htmlFor="f-dbuser">用户名</label>
                      <input
                        className="input mono"
                        id="f-dbuser"
                        type="text"
                        placeholder="root"
                        value={mysql.username}
                        onChange={(e) => patchMysql({ username: e.target.value })}
                      />
                    </div>
                    <div className="field" style={{ marginBottom: 0 }}>
                      <label htmlFor="f-dbpass">密码</label>
                      <input
                        className="input"
                        id="f-dbpass"
                        type="password"
                        placeholder="••••••••"
                        value={mysql.password}
                        onChange={(e) => patchMysql({ password: e.target.value })}
                      />
                    </div>
                  </div>
                  <div style={{ marginTop: 12, display: "flex", alignItems: "center", gap: 12 }}>
                    <button
                      className="btn btn-outline btn-sm"
                      type="button"
                      onClick={testConnection}
                      disabled={test.kind === "testing"}
                    >
                      {test.kind === "testing" ? "测试中…" : "测试连接"}
                    </button>
                    {test.kind === "ok" && (
                      <span style={{ color: "var(--ok)", fontSize: 13 }}>✓ {test.message}</span>
                    )}
                    {test.kind === "err" && (
                      <span style={{ color: "var(--err)", fontSize: 13 }}>✕ {test.message}</span>
                    )}
                  </div>
                </>
              )}
            </div>
          </label>
        </div>

        <div className="setup-actions">
          <button className="btn btn-primary" type="submit" disabled={saving}>
            {saving ? "初始化中…" : "下一步"}
          </button>
        </div>
      </form>
    </div>
  );
}

/* ---------- 第二步:创建创始管理员 ---------- */

function AdminStep({
  onBack,
  onDone,
}: {
  onBack: () => void;
  onDone: (admin: { username: string; nickname: string; password: string }) => void;
}) {
  const [username, setUsername] = useState("");
  const [nickname, setNickname] = useState("");
  const [password, setPassword] = useState("");
  const [password2, setPassword2] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!USERNAME_RE.test(username) || username.length > 64) {
      setError("用户名需为小写字母、数字或连字符,且不能以连字符开头/结尾(≤64 字符)");
      return;
    }
    if (!nickname.trim()) {
      setError("请输入昵称");
      return;
    }
    if (password.length < 8) {
      setError("密码至少 8 位");
      return;
    }
    if (password !== password2) {
      setError("两次输入的密码不一致");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const admin = { username, nickname: nickname.trim(), password };
      await setupApi.createAdmin(admin);
      onDone(admin);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "创建管理员失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="setup-card">
      <Steps current={2} />

      <h1>创建初始管理员</h1>
      <p className="sub">安装完成后,你可以用它登录并邀请更多用户。</p>

      <div className="alert warn" style={{ marginBottom: 20 }}>
        <span>!</span>
        <span>
          <b>该账号为创始用户。</b>拥有最高权限,创建后无法被删除、禁用或降级。
        </span>
      </div>

      <ErrorAlert message={error} />

      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="f-username">用户名</label>
          <input
            className="input mono"
            id="f-username"
            type="text"
            placeholder="admin"
            autoComplete="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <div className="hint">小写字母/数字/连字符,将成为命名空间,不可修改</div>
        </div>

        <div className="field">
          <label htmlFor="f-nickname">昵称</label>
          <input
            className="input"
            id="f-nickname"
            type="text"
            placeholder="管理员昵称"
            value={nickname}
            onChange={(e) => setNickname(e.target.value)}
          />
          <div className="hint">展示名,可随时修改</div>
        </div>

        <div className="field-2col">
          <div className="field">
            <label htmlFor="f-password">密码</label>
            <input
              className="input"
              id="f-password"
              type="password"
              placeholder="至少 8 位"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="f-password2">确认密码</label>
            <input
              className="input"
              id="f-password2"
              type="password"
              placeholder="再次输入密码"
              autoComplete="new-password"
              value={password2}
              onChange={(e) => setPassword2(e.target.value)}
            />
          </div>
        </div>

        <div className="setup-actions">
          <button className="btn btn-outline" type="button" onClick={onBack} disabled={submitting}>
            上一步
          </button>
          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? "安装中…" : "完成安装"}
          </button>
        </div>
      </form>
    </div>
  );
}

/* ---------- 第三步:完成摘要 ---------- */

function DoneStep({
  dbConfig,
  admin,
}: {
  dbConfig: DatabaseConfig;
  admin: { username: string; nickname: string };
}) {
  const dataDir = dbConfig.driver === "sqlite" ? dbConfig.data_dir.replace(/\/+$/, "") : "";
  const dbSummary =
    dbConfig.driver === "sqlite"
      ? `SQLite · ${dataDir || "/data"}/runemarket.db`
      : `MySQL · ${dbConfig.host}/${dbConfig.database}`;

  return (
    <div className="setup-card center">
      <Steps current={3} />

      <div className="setup-done-icon">✓</div>
      <h1>安装完成</h1>
      <p className="sub" style={{ marginBottom: 0 }}>
        RuneMarket 已就绪。
      </p>

      <div className="meta-list">
        <div className="row">
          <span className="k">数据库</span>
          <span className="v mono">{dbSummary}</span>
        </div>
        <div className="row">
          <span className="k">管理员</span>
          <span className="v">
            {admin.nickname} <span className="muted">(@{admin.username})</span>
          </span>
        </div>
        <div className="row">
          <span className="k">主密钥</span>
          <span className="v mono">
            /data/secret <span className="muted">(0600,已生成)</span>
          </span>
        </div>
        <div className="row">
          <span className="k">配置文件</span>
          <span className="v mono">/data/config.toml</span>
        </div>
      </div>

      {/* 安装完成后服务端切换到正常模式,整页跳转以重新探测 setup/status */}
      <a className="btn btn-accent" href="/login">
        进入 RuneMarket
      </a>
    </div>
  );
}
