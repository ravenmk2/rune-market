-- M2 schema: skill / skill_version / tag / skill_tag (design §6.2, SQLite dialect)

CREATE TABLE skill (
  id                CHAR(32)   PRIMARY KEY,
  owner_id          CHAR(32)   NOT NULL REFERENCES user(id),
  name              VARCHAR(64) NOT NULL,
  summary           VARCHAR(1024) NOT NULL DEFAULT '', -- 取自最新版本 description
  official          BOOLEAN     NOT NULL DEFAULT 0,
  status            VARCHAR(16) NOT NULL DEFAULT 'published', -- pending | published | taken_down
  latest_version_id CHAR(32),
  download_count    BIGINT      NOT NULL DEFAULT 0,
  created_at        TIMESTAMP   NOT NULL,
  updated_at        TIMESTAMP   NOT NULL,
  UNIQUE (owner_id, name)
);

CREATE TABLE skill_version (
  id            CHAR(32)   PRIMARY KEY,
  skill_id      CHAR(32)   NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
  version       VARCHAR(32) NOT NULL,        -- semver,不带 v 前缀
  description   TEXT        NOT NULL,
  license       VARCHAR(255) NOT NULL DEFAULT '',
  compatibility VARCHAR(500) NOT NULL DEFAULT '',
  author        VARCHAR(255) NOT NULL DEFAULT '',
  harnesses     TEXT        NOT NULL,        -- JSON 数组;[] 表示通用
  permissions   TEXT        NOT NULL,        -- JSON:解析后的 allowed-tools 结构化列表
  frontmatter   TEXT        NOT NULL,        -- JSON:frontmatter 原文透传
  file_count    INTEGER     NOT NULL DEFAULT 0,
  sha256        CHAR(64)    NOT NULL REFERENCES blob(sha256),
  size          BIGINT      NOT NULL,
  filename      VARCHAR(255) NOT NULL,
  created_at    TIMESTAMP   NOT NULL,
  UNIQUE (skill_id, version)
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
