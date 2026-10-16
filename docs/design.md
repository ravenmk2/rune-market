# RuneMarket 详细设计

> 本文档是开发依据:数据模型、API、管线、目录结构均以此为准;变更需同步更新本文。

## 1. 概述

RuneMarket 是 Agent Skills 与 DESIGN.md 的制品商店(类似 Docker Hub):

- 多用户,角色分管理员 / 普通用户;普通用户管理自己的制品,管理员另有独立管理面板(用户、全站制品、系统设置、官方制品标记)
- 制品命名空间 = 用户名(`runemarket/pdf-processing`),官方制品归属 `runemarket` 命名空间
- 单体分发:一个 Go 二进制(embed React 构建产物),首次运行进入安装向导
- 数据库可选 SQLite(默认)/ MySQL,安装向导中配置

## 2. 技术栈与关键依赖

| 层 | 选型 | 说明 |
| --- | --- | --- |
| 后端 | Go,Gin,Logrus | |
| 数据库 | SQLite(`modernc.org/sqlite` 纯 Go 驱动)/ MySQL(`go-sql-driver/mysql`) | CGO_ENABLED=0 基线,DSN 见 §6.1 |
| 前端 | React + TS + Vite,产物 embed 进二进制 | `web/dist/` 整体 gitignore |
| UUID | `github.com/google/uuid`(v1.6+ 支持 v7) | 主键时序 UUIDv7,去横线 32 字符,存 `CHAR(32)` |
| 密码 | `golang.org/x/crypto/bcrypt` | cost 默认 10 |
| YAML | `gopkg.in/yaml.v3` | SKILL.md frontmatter 解析 |
| 压缩 | 标准库 `archive/zip`、`archive/tar`+`compress/gzip` | 无需外部依赖 |
| 图片 | 标准库 `image/*` + `golang.org/x/image/draw` | 解码 PNG/JPG,统一重编码为 PNG |
| semver | `golang.org/x/mod/semver` | 版本号校验与排序 |
| 配置 | `github.com/BurntSushi/toml` | `./data/config.toml` |

模块路径:`github.com/ravenmk2/rune-market`。

## 3. 仓库目录结构与设计原则

### 3.1 设计原则:高内聚、低耦合、可扩展、不过度设计

- **按业务域分包,包间只暴露窄接口**:`store` 每实体一个 Store 接口、`blob` 暴露 `Storage` 接口、`skillpkg` 暴露 `Validator`/`Package` 接口;接口定义在**使用方**包内(Go 惯例),实现可替换
- **组装集中在 `main`**:依赖构造与注入只在 `cmd/runemarket/main.go`,业务包不做全局单例
- **扩展点明确、成本可控**:新增制品类型 = 新域包 + 新路由组 + 新表迁移;新增数据库方言 = `store` 新 dialect 目录;新增展示尺寸 = 尺寸白名单加一项
- **不做**:插件系统、事件总线、CQRS/分层架构套娃、ORM(GORM);用 `database/sql` + 手写 SQL,方言差异集中在 migrations 与少量 DSN/类型判断
- 前端同样:组件自包含(组件、样式、测试同目录),`api/` 按域分模块,共享的只有 token、基础组件与 client

### 3.2 目录结构

```txt
rune-market/
├── cmd/
│   └── runemarket/
│       └── main.go            # 组装与启动:config → secret → db → migrate → http
├── internal/
│   ├── config/                # ./data/config.toml 读写、安装状态判断
│   ├── secret/                # ./data/secret 主密钥生成与加载
│   ├── store/                 # 数据库层:migrations + 各实体 CRUD(双方言)
│   │   ├── migrations/
│   │   │   ├── sqlite/        # 0001_init.sql ...
│   │   │   └── mysql/
│   │   ├── driver_cgo.go      # (可选)CGO sqlite3 驱动
│   │   └── driver_nocgo.go    # 默认 modernc 纯 Go 驱动
│   ├── server/                # Gin 引擎、中间件、路由注册
│   ├── auth/                  # 注册/登录/会话/bcrypt/权限中间件
│   ├── skillpkg/              # skill 包解析:解压缩、frontmatter 校验、harness 检测
│   ├── designmd/              # DESIGN.md 校验(弱验证)
│   ├── blob/                  # 内容寻址存储(压缩包/图片,原格式保留)、缩略图派生
│   └── hub/                   # 业务编排:发布、版本、下架、官方标记、审核
├── web/
│   ├── embed.go               # //go:embed all:dist
│   ├── dist/                  # 前端构建产物(gitignore,CI 用占位文件)
│   └── src/                   # React 源码(Vite 工程)
├── scripts/
│   └── build.sh               # 本地多平台构建(版本注入、dist 占位)
├── .github/workflows/
│   ├── test.yml               # lint + test
│   └── release.yml            # goreleaser + GHCR 镜像
├── docs/
│   └── design.md              # 本文档
├── Dockerfile                 # 多阶段源码构建(web → app → runtime)
├── Dockerfile.release         # 仅 COPY goreleaser 产物,不重复编译
├── go.mod / .golangci.yml / .goreleaser.yml / Makefile
└── AGENTS.md / README.md / LICENSE
```

## 4. 数据目录布局

数据目录相对工作目录:`./data`(Docker 容器中为 `/app/data`,挂卷持久化)。

```txt
data/
├── config.toml          # 安装向导写入:数据库类型/连接、数据目录
├── secret               # 主密钥,32 字节随机 hex,0600,首次启动自动生成
├── runemarket.db        # SQLite 数据文件(仅 SQLite 模式)
├── blobs/               # skill 压缩包,文件名即 sha256,无扩展名(内容寻址,不可变)
│   └── <sha256>
├── images/              # 图片(DESIGN.md 预览图、skill 图标),保留原始格式(内容寻址,不可变)
│   ├── <sha256>.<ext>         # 原图(png/jpg 原字节,sha256 对原字节计算)
│   └── <sha256>_640.png       # 列表缩略图(宽 640 等比,统一 PNG)
└── avatars/             # 用户头像,按用户寻址(可变,覆盖式)
    ├── <user_id>.png          # 当前头像原图
    └── <user_id>_32.png       # 缩略图(32/64/128)
```

- 两类寻址模型:**不可变内容**(压缩包、预览图)内容寻址;**可变用户状态**(头像)按用户寻址,覆盖式写入,理由见 §11
- 派生缩略图带 `_<size>` 后缀,不入 `blob` 表,随原图删除
- 除 SQLite 数据文件外,业务数据(用户、制品元数据、DESIGN.md 内容)一律在数据库中

## 5. 启动流程与安装向导

```txt
启动
 ├─ ./data/config.toml 不存在 → setup 模式
 │    ├─ 仅注册 /setup 静态页与 /api/v1/setup/* 路由,其余全部 302 → /setup
 │    ├─ ① 选择数据库(SQLite / MySQL,MySQL 提供"测试连接")
 │    ├─ ② 创建创始管理员(用户名/昵称/密码)
 │    └─ ③ 写 config.toml、初始化 schema、生成 secret → 完成页
 └─ config.toml 存在 → 正常模式
      ├─ ./data/secret 不存在 → 生成(32B rand → hex,0600)
      ├─ 连接数据库,执行未应用的 migrations
      └─ 启动 HTTP(:8080)
```

- 安装完成后 `/setup` 与 `/api/v1/setup/*` 返回 404(以 config.toml 存在为准)
- 创始用户:`user.is_founder = true`,**不可删除、不可禁用、不可降级**,后端在对应接口硬校验
- MySQL 连接密码存 config.toml(0600);如需加密,用主密钥 AES-GCM,首版明文 + 文件权限即可

## 6. 数据模型

表名单数;主键时序 UUIDv7,**去横线 32 个十六进制字符**,应用层生成(`uuid.NewV7()` 后 `hex` 编码无分隔符),存 `CHAR(32)`。时间统一 UTC,精确到毫秒。

### 6.1 双方言策略

- 占位符两边都是 `?`;差异集中在 DDL 与少数类型判断,migrations 按方言分目录
- SQLite DSN:`file:data/runemarket.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)`
- MySQL DSN:`user:pass@tcp(host:3306)/runemarket?parseTime=true&charset=utf8mb4&collation=utf8mb4_bin`
- 字段类型按方言取最适合的(不追求两方统一):

| 逻辑类型 | SQLite | MySQL |
| --- | --- | --- |
| 主键/外键 UUID | `CHAR(32)` | `CHAR(32)` |
| sha256 | `CHAR(64)` | `CHAR(64)` |
| 布尔 | `BOOLEAN`(即 INTEGER) | `TINYINT(1)` |
| 枚举(role/status) | `VARCHAR(16)` + 应用层校验 | `VARCHAR(16)` + 应用层校验(不用 ENUM,避免迁移改枚举值) |
| JSON 列 | `TEXT`(应用层序列化) | `JSON` 原生 |
| 计数/大小 | `INTEGER` | `BIGINT UNSIGNED` / `INT UNSIGNED` |
| 时间 | `TIMESTAMP`(存 UTC) | `DATETIME(3)`(存 UTC) |
| 长文本(content) | `TEXT` | `MEDIUMTEXT` |
| 排序规则 | BINARY | `utf8mb4_bin`(username/name/tag 精确匹配) |

### 6.2 DDL(SQLite 方言)

```sql
CREATE TABLE user (
  id            CHAR(32)    PRIMARY KEY,
  username      VARCHAR(64) NOT NULL UNIQUE,     -- 命名空间,小写字母/数字/连字符
  nickname      VARCHAR(64) NOT NULL,            -- 展示名
  password_hash VARCHAR(100) NOT NULL,
  role          VARCHAR(16) NOT NULL DEFAULT 'user',   -- admin | user
  is_founder    BOOLEAN     NOT NULL DEFAULT 0,
  status        VARCHAR(16) NOT NULL DEFAULT 'active', -- active | pending | disabled
  has_avatar    BOOLEAN     NOT NULL DEFAULT 0,  -- 头像文件按 <user_id> 寻址,见 §11
  bio           TEXT        NOT NULL DEFAULT '',
  created_at    TIMESTAMP   NOT NULL,
  updated_at    TIMESTAMP   NOT NULL
);

CREATE TABLE session (
  id         CHAR(32)   PRIMARY KEY,
  user_id    CHAR(32)   NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  token_hash CHAR(64)   NOT NULL UNIQUE,   -- session token 的 sha256,token 只出现一次
  ip         VARCHAR(45) NOT NULL DEFAULT '',
  user_agent TEXT        NOT NULL DEFAULT '',
  expires_at TIMESTAMP  NOT NULL,
  created_at TIMESTAMP  NOT NULL
);
CREATE INDEX idx_session_user ON session(user_id);

CREATE TABLE blob (
  sha256     CHAR(64)   PRIMARY KEY,
  kind       VARCHAR(16) NOT NULL,           -- archive | image;决定存储目录(blobs/ 或 images/)
  size       BIGINT      NOT NULL,
  ext        TEXT        NOT NULL DEFAULT '', -- image 的原始格式扩展名(png/jpg),archive 为空
  ref_count  INTEGER     NOT NULL DEFAULT 1,
  created_at TIMESTAMP   NOT NULL
);

CREATE TABLE skill (
  id                CHAR(32)   PRIMARY KEY,
  owner_id          CHAR(32)   NOT NULL REFERENCES user(id),
  name              VARCHAR(64) NOT NULL,
  display_name      VARCHAR(64) NOT NULL DEFAULT '', -- 技能展示名称,默认 name,1–64 字符
  summary           VARCHAR(1024) NOT NULL DEFAULT '', -- 取自最新版本 description,发布后可编辑
  official          BOOLEAN     NOT NULL DEFAULT 0,
  status            VARCHAR(16) NOT NULL DEFAULT 'published', -- pending | published | taken_down
  latest_version_id CHAR(32),
  icon_sha256       CHAR(64)    REFERENCES blob(sha256), -- 可选图标(image blob),发布/编辑可设
  download_count    BIGINT      NOT NULL DEFAULT 0,
  created_at        TIMESTAMP   NOT NULL,
  updated_at        TIMESTAMP   NOT NULL,
  UNIQUE (owner_id, name)
);

CREATE TABLE skill_version (
  id            CHAR(32)   PRIMARY KEY,
  skill_id      CHAR(32)   NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
  version       VARCHAR(32) NOT NULL,        -- semver,不带 v 前缀
  description   TEXT        NOT NULL,             -- 当前版本可由 owner/admin 编辑
  license       VARCHAR(255) NOT NULL DEFAULT '',
  compatibility VARCHAR(500) NOT NULL DEFAULT '',
  author        VARCHAR(255) NOT NULL DEFAULT '',
  harnesses     TEXT        NOT NULL,        -- JSON 数组,如 ["claude-code","codex"];[] 表示通用
  permissions   TEXT        NOT NULL,        -- JSON:解析后的 allowed-tools 结构化列表
  frontmatter   TEXT        NOT NULL,        -- JSON:frontmatter 原文透传(含厂商扩展字段)
  file_count    INTEGER     NOT NULL DEFAULT 0,
  sha256        CHAR(64)    NOT NULL REFERENCES blob(sha256),
  size          BIGINT      NOT NULL,
  filename      VARCHAR(255) NOT NULL,
  created_at    TIMESTAMP   NOT NULL,
  UNIQUE (skill_id, version)
);

CREATE TABLE designmd (
  id                CHAR(32)   PRIMARY KEY,
  owner_id          CHAR(32)   NOT NULL REFERENCES user(id),
  name              VARCHAR(64) NOT NULL,
  summary           VARCHAR(1024) NOT NULL DEFAULT '', -- 表单手填
  official          BOOLEAN     NOT NULL DEFAULT 0,
  status            VARCHAR(16) NOT NULL DEFAULT 'published',
  latest_version_id CHAR(32),
  download_count    BIGINT      NOT NULL DEFAULT 0,
  created_at        TIMESTAMP   NOT NULL,
  updated_at        TIMESTAMP   NOT NULL,
  UNIQUE (owner_id, name)
);

CREATE TABLE designmd_version (
  id                     CHAR(32)   PRIMARY KEY,
  designmd_id            CHAR(32)   NOT NULL REFERENCES designmd(id) ON DELETE CASCADE,
  version                VARCHAR(32) NOT NULL,
  content                TEXT        NOT NULL,   -- DESIGN.md 全文,直接存库
  sha256                 CHAR(64)    NOT NULL,   -- 内容哈希(完整性/展示),不关联 blob
  preview_desktop_sha256 CHAR(64)    REFERENCES blob(sha256),
  preview_mobile_sha256  CHAR(64)    REFERENCES blob(sha256),
  preview_light_desktop_sha256 CHAR(64) REFERENCES blob(sha256),
  preview_light_mobile_sha256  CHAR(64) REFERENCES blob(sha256),
  preview_dark_desktop_sha256  CHAR(64) REFERENCES blob(sha256),
  preview_dark_mobile_sha256   CHAR(64) REFERENCES blob(sha256),
  created_at             TIMESTAMP   NOT NULL,
  UNIQUE (designmd_id, version)
);

CREATE TABLE tag (
  id   CHAR(32)    PRIMARY KEY,
  name VARCHAR(64) NOT NULL UNIQUE
);

CREATE TABLE skill_tag (
  skill_id CHAR(32) NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
  tag_id   CHAR(32) NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
  PRIMARY KEY (skill_id, tag_id)
);

CREATE TABLE designmd_tag (
  designmd_id CHAR(32) NOT NULL REFERENCES designmd(id) ON DELETE CASCADE,
  tag_id      CHAR(32) NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
  PRIMARY KEY (designmd_id, tag_id)
);

CREATE TABLE setting (
  key   VARCHAR(64) PRIMARY KEY,
  value TEXT        NOT NULL
);
```

### 6.3 关键规则

- **blob 引用计数**:仅覆盖不可变内容(skill 压缩包、预览图):`skill_version.sha256` 与预览图引用时 `ref_count+1`(同事务);删除版本/制品时 `-1`,归零按 `kind` 删除对应目录文件及派生缩略图。**头像不入 blob 表**(按用户寻址、覆盖式),删除用户时直接删除其头像文件组
- **存储统计**:`SELECT SUM(size) FROM blob` + `avatars/` 目录扫描(头像量小,实时扫描即可)
- **latest 指针**:发布新版本时更新 `latest_version_id`;下架/删除最新版本后回退到剩余最高 semver
- **下载计数**:下载接口内 `download_count+1`(允许近似,不做事件表)
- **setting 键**:`site_name`、`site_description`、`site_tagline`(首页标语,≤200,可空)、`page_size`、`registration_mode`(open/approval/closed,安装后默认 closed)、`artifact_review`(none/required)、`upload_max_mb`、`anonymous_browse`、`anonymous_download`

## 7. 认证、会话与权限

- **注册**:用户名(正则 `^[a-z0-9](-?[a-z0-9])*$`,≤64)+ 昵称 + 密码(≥8)。按 `registration_mode`:open → 直接 active;approval → `pending`,管理员批准后可用;closed → 拒绝(仅管理员后台创建)
- **登录**:用户名 + 密码,bcrypt 校验;成功则签发会话
- **会话**:opaque token(32B rand,base64)写入 HttpOnly + SameSite=Lax Cookie(14 天,滑动续期);库中只存 `sha256(token)`;登出删 session 行
- **CSRF**:Cookie 会话 + 修改类请求校验自定义头 `X-Requested-With: XMLHttpRequest`(SPA 统一携带),SameSite=Lax 双保险;接口均为 JSON/二进制 API,不引入额外 token
- **权限中间件**:`requireAuth` / `requireAdmin`;资源级校验(制品 owner 或 admin);创始保护:`is_founder` 目标在 删除/禁用/降级 接口直接 403
- **密码哈希**:bcrypt cost 10;管理员"重置密码"生成随机临时密码,展示一次,用户首次登录强制修改(首版仅提示,不强制)

## 8. API 设计

统一前缀 `/api/v1`;错误格式 `{"error": {"code": "...", "message": "...", "details": [...]}}`;列表统一 `{"items": [...], "total": n, "page": n, "page_size": n}`。

**上传约定:不使用 multipart。文件数据即请求体(raw body)**,一次请求一个文件;图片经 `POST /images`、skill 压缩包经 `POST /archives` 预上传换取 sha256,发布/编辑请求一律为 **JSON**,以 sha256 引用已上传 blob(服务端不信任客户端,发布时对实包重新校验)。

### 8.1 安装向导(仅 setup 模式)

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/setup/status` | 当前步骤与模式 |
| POST | `/setup/database/test` | MySQL 连接测试(JSON) |
| POST | `/setup/database` | 保存数据库配置并初始化 schema(JSON) |
| POST | `/setup/admin` | 创建创始用户,完成安装(JSON;写 config.toml,生成 secret) |

### 8.2 认证与账号

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/register` | 注册(JSON;受 registration_mode 约束) |
| POST | `/auth/login` | 登录(JSON),种会话 Cookie |
| POST | `/auth/logout` | 登出 |
| GET | `/auth/me` | 当前用户(含 role、has_avatar,前端据此显示管理入口与头像) |
| GET | `/users/{username}` | 用户公开主页(昵称、bio、其已发布制品) |
| PUT | `/account` | 改昵称、bio(JSON) |
| PUT | `/account/password` | 改密码(JSON;校验旧密码,作废旧会话) |
| POST | `/account/avatar` | 上传头像:**raw body = 图片**(PNG/JPG ≤5MB),归一化 PNG 覆盖写入 `avatars/`,生成 32/64/128 缩略图,置 `has_avatar` |
| DELETE | `/account/avatar` | 移除头像(删除文件组,清 `has_avatar`,回落字母占位) |

### 8.3 市场(匿名可浏览受 `anonymous_browse` 控制)

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/site` | 站点名称/描述/首页标语/版本号(公开,顶栏品牌与页脚读取);返回 `mode=normal`,**setup 模式下也可达**(返回 `mode=setup` 与 `step`),SPA 启动只探测这一个端点 |
| GET | `/skills?q=&tag=&official=&sort=&page=` | 列表(卡片所需字段,含 `icon_url`) |
| GET | `/skills/tags` · `/designs/tags` | 各类已发布制品的完整标签集合 `{items: string[]}`,去重按名称排序,不随搜索/分页/标签筛选改变;遵循 `anonymous_browse`,不包含仅被待审核或下架制品使用的标签 |
| GET | `/skills/{ns}/{name}` | 详情(latest 版本元数据 + 统计 + `icon_url`) |
| GET | `/skills/{ns}/{name}/versions` | 版本列表 |
| GET | `/skills/{ns}/{name}/versions/{ver}` | 指定版本元数据 |
| GET | `/skills/{ns}/{name}/versions/{ver}/files` | 包内目录树(从 blob 流式读 zip/tar 生成) |
| GET | `/skills/{ns}/{name}/versions/{ver}/file?path=` | 单文件内容(文本直出;单文件上限 1MB) |
| GET | `/skills/{ns}/{name}/versions/{ver}/download` | 下载原始压缩包(`anonymous_download` 控制) |
| GET | `/designs?...` | 以上对称:DESIGN.md 列表/详情/版本 |
| GET | `/designs/{ns}/{name}/versions/{ver}/content` | 渲染用 Markdown 原文(从 DB) |
| GET | `/designs/{ns}/{name}/versions/{ver}/download` | 下载 DESIGN.md(text/markdown,从 DB 吐) |
| GET | `/images/{sha256}.{ext}` / `/images/{sha256}_640.png` | 图片原图(png/jpg,按扩展名给 Content-Type)与派生缩略图(immutable 缓存) |
| GET | `/avatars/{user_id}.png` / `/avatars/{user_id}_{size}.png?v=` | 头像;`v` 取用户 `updated_at`,变更即失效 |

### 8.4 发布与我的制品(登录)

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/images` | 图片上传:**raw body = PNG/JPG**(≤5MB,保留原格式),返回 `{sha256, ext, size, url, thumb_url}`;供预览图与 skill 图标预上传 |
| POST | `/archives` | skill 压缩包上传:**raw body**(≤`upload_max_mb`),落 blob(内容寻址去重)后 Inspect,返回 `{sha256, size, report, metadata}`;多阶段发布第一步,上传页即时报错 |
| POST | `/skills` | 正式发布:**JSON** `{archive, version, skill_id?, display_name?, tags?, description?, icon?}`;`archive` 为 `/archives` 所得 sha256(须存在且 kind=archive,服务端重新 Inspect,报告有 error 拒绝);`icon` 为 `/images` 所得 sha256(可空,须 kind=image);同名制品存在 → 新版本;`display_name` 去除首尾空白后≤64 字符,新制品未提供/空值默认包内 name,已有制品未提供/空值保留原展示名称;`description` 非空时替代包内 description(同时写入 summary 与当前版本描述,≤1024) |
| POST | `/designs/validate` | **JSON** `{name, content}` 干跑,返回弱验证报告(发布页预检) |
| POST | `/designs` | **JSON** `{content, name, summary?, version, tags?, preview_desktop?, preview_mobile?}`;preview_* 为先前 `POST /images` 得到的 sha256(可空) |
| GET | `/mine/skills` / `/mine/designs` | 我的制品 |
| PUT | `/skills/{id}` | 改标签(JSON;skill 还可改 display_name(去除首尾空白后 1–64 字符)、summary、当前版本 description 与 `icon`——sha256 设置 / `""` 清除 / 缺省不变;designmd 可改 summary) |
| POST | `/skills/{id}/takedown` `/restore` | 下架/恢复(designmd 对称) |
| DELETE | `/skills/{id}` / `/designs/{id}` | 删除(级联版本,事务内减 blob 引用,含图标) |

`POST /skills` 的 `skill_id` 可选：指定时只向该技能新增版本，技能必须存在且属于当前用户（管理员也不能代替作者发布），服务端重新 Inspect 后核对包内 `name`；新版本必须按 SemVer 优先级高于该技能全部历史版本，重复版本返回 `409 version_exists`，其他不递增版本返回 `400 invalid_argument`。绑定发布在事务内锁定技能并读取版本（MySQL 使用当前锁定读，避免旧快照），校验失败不改变制品或 blob 引用。展示名称缺省/空值、图标缺省、标签缺省均沿用原值；显式 `tags: []` 清空标签。未传 `skill_id` 保持旧客户端的同名 upsert 与标签替换行为，无新增 API 路由或数据库迁移。

管理员发布的新制品默认 `official=1`(普通用户为 0);事后仍可经管理面板 official/unofficial 端点调整。

### 8.5 管理面板(`requireAdmin`)

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/admin/overview` | 统计卡(用户数、制品数、存储用量)+ 待办计数 |
| GET | `/admin/users?status=&q=&page=` | 用户列表 |
| POST | `/admin/users` | 新建用户(用户名/昵称/初始密码/角色) |
| POST | `/admin/users/{id}/approve` · `/disable` · `/enable` | 状态操作(创始保护) |
| POST | `/admin/users/{id}/role` | 设为管理员/取消管理员(创始保护) |
| POST | `/admin/users/{id}/reset-password` | 重置为临时密码 |
| DELETE | `/admin/users/{id}` | 删除用户(创始保护;其制品级联下架,头像文件组删除) |
| GET | `/admin/skills?status=&official=` / `/admin/designs?...` | 全站制品 |
| POST | `/admin/skills/{id}/official` · `/unofficial` | 官方标记(designmd 对称) |
| POST | `/admin/skills/{id}/approve` | 审核通过(artifact_review=required 时) |
| GET/PUT | `/admin/settings` | 系统设置读写 |

## 9. Skill 发布与校验管线

多阶段发布:`POST /archives` 上传(raw body,≤`upload_max_mb` 默认 20MB)→ 流式写入临时文件并同时计算 sha256 → 落 blob(`kind=archive`,内容寻址去重)→ 对实包依次执行:

1. **解包探测**(只读流式,不落盘):按魔数识别 zip / tar / tar.gz;拒绝:路径穿越(`..`、绝对路径)、符号链接、单文件 >50MB、文件总数 >2000、解压后总量 >200MB(zip bomb 防护)
2. **定位 SKILL.md**:压缩包根直接含 `SKILL.md`,或唯一顶层目录含 `SKILL.md`;两者都不是 → 报错
3. **frontmatter 解析与硬校验**(yaml.v3;任一失败即拒绝):
   - 仅对规范 6 字段做规则校验:`name`(必填,≤64,`^[a-z0-9](-?[a-z0-9])*$`,无连续连字符)、`description`(必填,≤1024)、`compatibility`(≤500);`license`/`allowed-tools`/`metadata` 存在即可
   - 存在顶层目录形态时,`name` 必须与目录名一致
   - **扩展字段不拒绝**:记录为提示项("检测到扩展字段 `when_to_use`,将原样保留")
4. **结构提示(警告不拦截)**:SKILL.md 正文 >500 行建议拆分;缺 `references/`/`scripts/` 时按内容特征提示
5. **元数据归一化提取**(写入一等列):description、license、compatibility、author(`metadata.author` → 发布者昵称兜底)、version 以表单输入为准(`metadata.version` 仅提示)
6. **权限解析**:`allowed-tools` 按空格切分为结构化列表(如 `Bash(python3:*)`),标注风险等级(`Bash`/`Write`/`Edit` 为警示,`Read` 类为安全),存 `permissions` JSON,详情页公示
7. **harness 检测**(见下表)→ `harnesses` JSON;空 = 通用
8. **落库**(`POST /skills` JSON 引用 archive sha256,单事务):校验 blob 存在且 `kind=archive` → 从 blob 路径重新 Inspect(报告有 error 拒绝,不信任客户端)→ AddRef 绑定引用 → skill upsert(按 `(owner_id, name)`)→ skill_version 插入(版本冲突报错)→ 更新 latest 指针与 summary → tag 关联 → 可选 icon 绑定(须 `kind=image`,AddRef 并写 `skill.icon_sha256`,已有旧图标则交换引用)

**harness 检测规则**:

| 检测信号 | 判定 |
| --- | --- |
| frontmatter 仅规范 6 字段 | 通用(空数组) |
| 含 `hooks`/`model`/`context`/`agent`/`argument-hint`/`user-invocable`/`disable-model-invocation`/`disallowed-tools`/`when_to_use`/`shell`/`background`/`effort`/`arguments` 之一 | + `claude-code` |
| 包内含 `agents/openai.yaml` | + `codex` |
| 含 `icon`/`color`/`paths` 之一 | + `cursor` |

**文件预览**:目录树与单文件内容均从 blob 流式读取(不解压落盘);文本按扩展名判断,前端高亮;`agents/openai.yaml` 等附属文件原样出现在树中。

## 10. DESIGN.md 发布与校验管线

1. 发布与预检均为 **JSON**:`POST /designs` 收 `{content, name, summary?, version, tags?, previews?}`,`POST /designs/validate` 收 `{name, content}` 干跑;content ≤1MB;name(必填,同 name 规则,命名空间下唯一)、version(semver)、summary、tags。`previews` 为 `{theme, device, sha256}` 数组,`theme=default|light|dark`(默认/浅色/深色),`device=desktop|mobile`,每个组合最多一张,共最多六张,均可选。图片预先 `POST /images` 换取 sha256(各 ≤5MB,PNG/JPG,保留原格式)。未知枚举、重复组合、非法哈希或非 image blob 拒绝发布。兼容旧 `preview_desktop`/`preview_mobile` 请求并映射默认组,非空旧字段不能与 `previews` 同时提交。
2. **弱验证(警告不拦截)**:Markdown 可解析;检测常见章节(Overview/Colors/Typography/Spacing/Components/Elevation/Guidelines),缺失给建议;Colors 章节尝试提取 hex 色值计数(用于校验报告"识别出 N 个颜色定义")
3. `content` 与 `sha256` 直接写 `designmd_version`;预览图 blob 引用(`kind=image`)+ 生成 `_640` 列表缩略图
4. 详情页"内容"tab 由前端渲染 Markdown;色板(色值+用途)首版由作者在正文中书写,前端对 hex 做行内色块增强(纯前端,可选增强)
5. **主题由标签描述**:`light`/`dark`/`mixed`/`neutral` 等沿用普通标签与筛选,不新增主题能力或默认主题字段,不根据标签/正文推断图片分组。一版本仍是一份 DESIGN.md,内容/下载不随预览切换改变。混合、中性或未分类图片放在“默认”组。
6. **固定六个预览槽位**:原 `preview_desktop_sha256`/`preview_mobile_sha256` 为默认组,新增四个 nullable 外键存浅色/深色桌面与移动图。迁移仅加列,不修改旧图片、正文、哈希或 blob 引用计数。发布在同一事务中为每个已填槽位增加引用(同一图片填多个位置按位置计数),删除制品时释放所有版本的六个槽位引用。每次发布完整指定该版本图片,不从上一版本继承。
7. **读取契约**:详情 latest、版本列表与单版本均增加 `previews: [{theme, device, url, thumb_url}]`(只含已上传图片,无图片返回 `[]`)与 `preview_cover_url`;旧 `preview_desktop_url`/`preview_mobile_url` 仍仅表示默认组。列表 `preview_thumb_url` 和详情封面按“默认桌面→浅色桌面→深色桌面→默认移动→浅色移动→深色移动”取首张,无图片返回 null。图片扩展名批量查询,封面从列表 SQL 一并读取,避免逐条查询。

## 11. 文件与图片存储:两种寻址模型

按生命周期分两类,各用最合适的模型:

| | 不可变内容(压缩包、图片) | 可变用户状态(头像) |
| --- | --- | --- |
| 寻址 | 内容寻址:`blobs/<sha256>`、`images/<sha256>.<ext>`(ext = png/jpg) | 用户寻址:`avatars/<user_id>.png` |
| 写入 | 已存在仅 `ref_count+1`(秒传) | 覆盖式,同一路径直接替换 |
| 删除 | `ref_count` 归零删文件及派生缩略图 | 删除用户 / 移除头像时删文件组 |
| 记账 | 入 `blob` 表(kind/ref_count/size/ext) | 不入表(`user.has_avatar` 标记) |
| 缓存 | `Cache-Control: public, max-age=31536000, immutable` | 短缓存 + `?v=<updated_at>` 变更即失效 |

- **图片保留原格式**:上传 PNG/JPG → 魔数与 `image.DecodeConfig` 预检(尺寸 ≤8192×8192,防 decompression bomb)→ 解码(供缩略图)→ **原字节**落盘 `<sha256>.<ext>`,sha256 对原字节计算,ext 记入 `blob` 列;URL 带扩展名,Content-Type 按扩展名,安全性由全局 `X-Content-Type-Options: nosniff` 兜底
- **缩略图**:上传时同步生成,`x/image/draw` 高质量缩放;头像中心裁剪正方形后缩 32/64/128;预览图/图标按宽 640 等比(`_640.png`,统一 PNG);派生文件不入库,随原图增删
- **头像尺寸白名单**:32(顶栏/表格)、64(用户主页)、128(账号设置);预览图仅 `_640`
- **上传引用语义**:`POST /images`/`POST /archives` 上传即持 1 份引用;发布/绑定(预览图、skill 图标、版本)再加 1;删除制品、清除/更换图标释放绑定引用。上传后未绑定的孤儿引用沿用现状不回收

## 12. 前端工程

`web/`:Vite + React + TS + react-router;样式从原型迁移为设计 token(CSS 变量)+ 组件样式,不引入 UI 框架。

```txt
web/src/
├── main.tsx / App.tsx
├── api/                   # 按域分模块:client.ts(fetch 封装) + auth.ts / skills.ts / designs.ts / admin.ts
├── styles/app.css         # 设计 token 与基础样式
├── components/            # 自包含组件(组件 + 样式 + 测试同目录):
│                          # Topbar / Card / Tabs / Pagination / Badge / Panel /
│                          # FileTree / CodeView / CheckList / Dropzone / Avatar ...
├── pages/
│   ├── marketplace/       # SkillsPage / DesignsPage
│   ├── skill/             # SkillDetail(说明|文件|版本 三 tab 路由)
│   ├── design/            # DesignDetail(预览|内容)
│   ├── publish/           # PublishSkill / PublishDesign
│   ├── auth/              # Login / Register
│   ├── mine/              # MySkills / MyDesigns / EditSkill / EditDesign
│   ├── account/           # AccountSettings
│   ├── user/              # UserProfile
│   ├── setup/             # SetupWizard(三步,仅 setup 模式可达)
│   └── admin/             # AdminLayout(通栏+侧栏)/ Overview / Users / UserNew /
│                          # AdminSkills / AdminDesigns / Settings
└── ...
```

- 路由:`/`(Skills)、`/designs`、`/s/{ns}/{name}[/files|/versions]`、`/d/{ns}/{name}[/content]`、`/publish`、`/publish/skills/{ns}/{name}`(技能新版本)、`/mine/...`、`/u/{username}`、`/account`、`/setup`、`/admin/...`
- 上传:fetch `body: Blob`(raw),不用 FormData;`api/client.ts` 统一封装:错误格式、`X-Requested-With`、401 跳登录
- DESIGN.md 发布:默认/浅色/深色三个上传分组,每组两个设备槽位,可替换或移除。逐槽位显示上传状态与错误,分组标签提示已添加/上传中/失败;任意图片上传中禁止发布。预览图随版本发布,编辑页仍仅改简介/标签。
- DESIGN.md 详情预览:主题选项由实际图片决定,多组显示切换按钮,仅浅色/深色一组显示名称,只有默认组隐藏主题控件。设备切换使用可访问的按钮,缺图设备禁用并说明;切换主题时保留设备,若该主题缺此设备则切换到该主题已有设备。首次进入按默认→浅色→深色选组,组内桌面优先;历史版本切换尽量保留当前主题/设备,不可用时重新选择,不会将其他组图片显示在错误标签下。站点自身主题不影响截图选择;无图片显示空状态。发布页两列槽位在窄屏改为单列,预览控件允许换行。
- Markdown 表格:净化后为 Markdown 和内嵌 HTML 表格添加可键盘聚焦的独立横向滚动容器;外框使用与代码块相同的 `--radius-sm` 圆角,白色内容区、浅色表头,外框与单元格分隔线不叠加。宽表格在自身容器滚动,不撑宽正文;保留表格语义及原有对齐。
- 市场筛选:`q`/`tag`/`official`/`page` 保存在 URL,刷新与浏览器前进后退可恢复条件;列表卡片向详情传递来源地址,详情各 tab 保留来源,面包屑可返回原筛选。标签从独立接口读取,筛选结果为空或翻页不导致筛选项消失;加载失败提供重试。卡片标题和作者使用独立链接,可通过键盘访问。
- 响应式:≤640px 市场单列,管理/我的发布表格以带字段名称的卡片呈现,操作按钮可换行;统计、长路径及元数据不撑宽页面。Skills/DESIGN.md 共用详情头部适配,≤760px 操作区独占一行;文件目录改为可折叠区域,选择文件后收起并将焦点交给正文。桌面保留三列市场和表格式管理布局。
- 可访问性与发布反馈:文件目录、预览/Raw、设置开关均使用带名称及状态的原生按钮,上传区支持 Enter/空格且有禁用态,交互控件提供键盘焦点样式。上传区就近显示校验摘要并可跳到完整报告;表单格式错误聚焦相应字段。连续选择 DESIGN.md 文件仅采纳最后一次校验结果。用户提示仅保留格式、大小、裁剪等实际使用信息。
- 技能展示:列表、我的发布、后台列表、详情均以 `display_name` 为主,同时展示原 `name`(地址、唯一性、下载文件名仍使用原名称);搜索覆盖展示名称、原名称与简介。发布页必填展示名称,上传校验完成后默认填入包内 `name`,用户已编辑时保留输入;编辑页可修改展示名称。迁移为历史技能回填 `display_name = name`。列表/详情 API 返回 `display_name`。
- 技能列表:市场、用户主页及管理列表均不展示技能简介/说明;详情页保留版本说明,简介仍存储并参与搜索。
- 新版本发布：我的发布与作者自己的详情页提供“发布新版本”入口；显示目标展示名称、原名称和最高语义版本，默认建议补丁号 +1，最高为预发布时建议对应正式版本，并提供次版本/主版本建议及手动 SemVer 输入（含预发布与构建信息）。展示名称、图标、标签预填并折叠，说明从新包提取；包名不匹配阻止提交。普通发布页检查同命名空间下是否已有该技能，存在则引导使用新版本入口，查询失败不视为新技能。
- 商店文案：Skills 与 DESIGN.md 首页标题为“智能体技能与设计系统商店”，顶部导航与详情面包屑使用“浏览”，后台返回按钮为“返回商店”，相关提示同步。卡片下载量使用共用 `DownloadCount` SVG 图标与缩写数字，悬停/无障碍标签显示完整下载次数。
- 技能默认图标:未上传自定义图片时,技能卡片、详情、发布和编辑表单统一使用暖橙色“文档 + 星芒”SVG(`SkillIcon`),不使用名称首字母或轮换底色。
- 头像组件:`has_avatar` 为真输出 `/avatars/{id}_{size}.png?v={updated_at}`,否则字母占位
- 鉴权态:`GET /auth/me` 全局 Context;admin 路由前端守卫 + 后端强制
- 开发:Vite dev server 代理 `/api`、`/images`、`/avatars` 到 Go(:8080);生产走 embed SPA 回退(静态命中则直出,否则回退 index.html;API 路由先注册)

## 13. 安全设计

- 主密钥 `./data/secret`(0600,32B hex):config 敏感字段加密、未来 token 签名的备用密钥;重新生成会使全站会话失效(管理面板"重新生成"按钮标注谨慎)
- 上传全链路限制:大小、魔数探测、路径穿越、符号链接、zip bomb、单文件/总数上限;图片解码前 `DecodeConfig` 预检尺寸
- 文件预览:文本超 1MB 截断;一律 `Content-Type: text/plain; charset=utf-8` 或强制下载,不按原始类型内联(防 XSS)
- DESIGN.md 内容渲染:前端 Markdown 渲染必须 sanitize(marked + DOMPurify)
- bcrypt cost 10;登录与注册接口按 IP 限流(内存令牌桶,10 次/分钟)
- 安全响应头:X-Content-Type-Options、X-Frame-Options=DENY、Referrer-Policy

## 14. 工程化

- **构建基线**:`CGO_ENABLED=0` 纯 Go 构建,全平台交叉编译;SQLite 默认 `modernc.org/sqlite` 纯 Go 驱动,保留 CGO 驱动切换文件(`driver_cgo.go`/`driver_nocgo.go` 按 build constraint 自动选择)
- **版本注入**:`-ldflags "-s -w -X main.version=<ver>"`,代码内 `var version = "dev"` 兜底;release 取 git tag 去 `v` 前缀,本地构建用 `git describe --tags --always --dirty`
- **lint**:golangci-lint v2,默认集 + `misspell`/`unconvert`/`gofmt`/`goimports`;CI 固定小版本,且其构建 Go 版本必须 ≥ go.mod 目标版本,升级 Go 工具链时同步升级
- **Test CI**(`.github/workflows/test.yml`):push 主干分支 + PR + 手动触发;lint job 与 test job 并行;test 矩阵 ubuntu/windows/macos,`CGO_ENABLED=0`,先 `go vet` 后 `go test`;Go 版本经 `go-version-file: go.mod` 读取;两 job 编译前创建 `web/dist` 占位文件(`mkdir -p web/dist && touch web/dist/index.html`)
- **dev 镜像**(`.github/workflows/docker-dev.yml`):push master/main/dev/develop + 手动触发;自带 lint + ubuntu 单平台 test 作为门禁(`needs` 串行,不复制三 OS 矩阵),通过后由 docker job 经 buildx 源码构建(Dockerfile 多阶段,`VERSION=$(git describe --tags --always --dirty)`,checkout `fetch-depth: 0` 取 tag 历史,与本地/发版口径一致)推送 `ghcr.io/ravenmk2/rune-market:dev`(仅 linux/amd64,无需 QEMU);`packages: write` 仅授予 docker job;同分支并发取消(cancel-in-progress)
- **release**(`.goreleaser.yml` + `.github/workflows/release.yml` 两件套):tag `v*.*.*` 触发;goreleaser 固定 `~> v2`;linux/darwin/windows × amd64/arm64 交叉编译,windows 产物 zip、其余 tar.gz,产物名 `<name>_<version>_<os>_<arch>` + checksums.txt;changelog 从 Conventional Commits 生成(Features/Bug Fixes 两组,排除 docs/test/chore/ci/style/build 与合并提交),不维护 CHANGELOG.md;release 前先完成前端真实构建(dist 为真实产物)
- **容器镜像**:双 Dockerfile 分工——`Dockerfile` 多阶段源码构建(node:24-alpine 前端 → golang:1.26-alpine 后端,`ARG VERSION/GOPROXY` 可调,供自托管用户 `docker build` 一键构建);`Dockerfile.release` 供 goreleaser `dockers_v2`(buildx 单段配置直接产出多架构 manifest)打包已交叉编译的二进制(与归档产物同源同构建,不在镜像内重复编译),GHCR 多架构 manifest,tag 为版本号(无 `v`)+ `latest`;两文件运行时层一致:alpine + ca-certificates,`COPY --chmod=755 ${TARGETPLATFORM}/runemarket /usr/local/bin/`(`Dockerfile` 为上下文根目录的 `runemarket`);容器内 `WORKDIR /app`,数据目录 `/app/data`(挂卷),EXPOSE 8080
- **本地构建**:`scripts/build.sh`(Git Bash 可执行):dist 占位 → 版本注入 → 六平台产物到 `dist/`;Makefile 仅提供 `lint` 便捷目标
- **验证**:`goreleaser check` + `goreleaser release --snapshot --clean` 本地快照验证

## 15. 里程碑

| 里程碑 | 内容 | 验收 |
| --- | --- | --- |
| M1 骨架与安装 | 仓库骨架、config/secret、双方言 store+migrations、安装向导、注册/登录/会话 | 全新环境跑通向导,SQLite/MySQL 均可登录 |
| M2 Skill 闭环 | 上传校验管线、发布/版本、市场列表/详情三 tab、文件预览、下载、我的发布/编辑 | 真实 skill 包全流程,校验报告符合 §9 |
| M3 DESIGN.md 闭环 | 发布(含预览图与缩略图)、预览/内容 tab、下载 | 同上,符合 §10 |
| M4 管理面板 | 概览统计、用户管理(创始保护)、制品管理(官方/下架/审核)、系统设置 | 权限矩阵全覆盖 |
| M5 打磨 | 头像与缩略图、搜索与标签筛选、审核流、限流、CI/release/GHCR 镜像 | 容器一键起,向导到可用 |

## 16. 决策记录(已确认)

1. 表名单数;主键时序 UUIDv7 去横线 32 字符,`CHAR(32)`,应用层生成
2. skill 与 designmd 实体分离;各自多版本,`(owner_id, name)`、`(*_id, version)` 唯一
3. 文件存储按生命周期分两种寻址模型:不可变内容(压缩包/预览图)内容寻址并入 `blob` 表记账;可变用户状态(头像)按 `<user_id>` 寻址、覆盖式、不入表,缓存用 `?v=updated_at` 失效
4. ~~图片统一归一化为 PNG~~(已推翻)图片**保留原始格式**(PNG/JPG 原字节落盘,ext 入 blob 列,URL 带扩展名);压缩包无扩展名;派生缩略图带 `_<size>` 后缀(预览图 `_640` 统一 PNG);圆角一律前端 CSS
5. DESIGN.md 内容存数据库(`designmd_version.content`),不入文件存储
6. 上传不使用 multipart:图片/压缩包分别经 `POST /images`、`POST /archives` raw body 预上传换 sha256,发布/编辑请求为 JSON 引用(多阶段发布;弃用 `POST /blobs`、`POST /skills/validate`);skill 支持自定义图标(`skill.icon_sha256` 引用 image blob)
7. 公共元数据一等列 + frontmatter 原文 JSON 透传;规范 6 字段硬校验,厂商扩展仅提示
8. harness 自动检测,命名 `harnesses`,空 = 通用
9. 注册三模式(open/approval/closed)存 setting;创始用户由安装向导创建,不可删除/禁用/降级
10. 数据库配置存 `./data/config.toml`(鸡生蛋问题),主密钥 `./data/secret` 0600
11. 无邮箱体系(验证码成本);用户名 = 命名空间,昵称为展示名
12. 数据目录 `./data`(Docker:`/app/data`)
13. 包间窄接口、main 集中组装;不做插件系统/事件总线/ORM
14. 管理面板通栏布局;表单主按钮左对齐、行内操作右对齐
15. 首版不做 Agent 拉取 API、不做下载量以外的统计
16. 双 Dockerfile:`Dockerfile` 多阶段源码构建(自托管一键构建),`Dockerfile.release` 仅打包 goreleaser 产物(release 同源);运行时层保持一致
