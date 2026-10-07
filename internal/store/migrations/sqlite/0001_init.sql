-- M1 schema: user / session / blob / setting (design §6.2, SQLite dialect)

CREATE TABLE user (
  id            CHAR(32)    PRIMARY KEY,
  username      VARCHAR(64) NOT NULL UNIQUE,     -- 命名空间,小写字母/数字/连字符
  nickname      VARCHAR(64) NOT NULL,            -- 展示名
  password_hash VARCHAR(100) NOT NULL,
  role          VARCHAR(16) NOT NULL DEFAULT 'user',   -- admin | user
  is_founder    BOOLEAN     NOT NULL DEFAULT 0,
  status        VARCHAR(16) NOT NULL DEFAULT 'active', -- active | pending | disabled
  has_avatar    BOOLEAN     NOT NULL DEFAULT 0,
  bio           TEXT        NOT NULL DEFAULT '',
  created_at    TIMESTAMP   NOT NULL,
  updated_at    TIMESTAMP   NOT NULL
);

CREATE TABLE session (
  id         CHAR(32)   PRIMARY KEY,
  user_id    CHAR(32)   NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  token_hash CHAR(64)   NOT NULL UNIQUE,
  ip         VARCHAR(45) NOT NULL DEFAULT '',
  user_agent TEXT        NOT NULL DEFAULT '',
  expires_at TIMESTAMP  NOT NULL,
  created_at TIMESTAMP  NOT NULL
);
CREATE INDEX idx_session_user ON session(user_id);

CREATE TABLE blob (
  sha256     CHAR(64)   PRIMARY KEY,
  kind       VARCHAR(16) NOT NULL,           -- archive | image
  size       BIGINT      NOT NULL,
  ref_count  INTEGER     NOT NULL DEFAULT 1,
  created_at TIMESTAMP   NOT NULL
);

CREATE TABLE setting (
  key   VARCHAR(64) PRIMARY KEY,
  value TEXT        NOT NULL
);
