-- M3 schema: designmd / designmd_version / designmd_tag (design §6.2, SQLite dialect)

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
  sha256                 CHAR(64)    NOT NULL,   -- 内容哈希,不关联 blob
  preview_desktop_sha256 CHAR(64)    REFERENCES blob(sha256),
  preview_mobile_sha256  CHAR(64)    REFERENCES blob(sha256),
  created_at             TIMESTAMP   NOT NULL,
  UNIQUE (designmd_id, version)
);

CREATE TABLE designmd_tag (
  designmd_id CHAR(32) NOT NULL REFERENCES designmd(id) ON DELETE CASCADE,
  tag_id      CHAR(32) NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
  PRIMARY KEY (designmd_id, tag_id)
);
