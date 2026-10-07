-- M1 schema: user / session / blob / setting (design §6.2, MySQL dialect per §6.1)
-- NOTE: TEXT columns cannot have DEFAULT in MySQL; the app always writes them.
-- NOTE: `key` is a reserved word in MySQL and must stay backticked.
-- NOTE: `blob` collides with the BLOB type keyword and must stay backticked.

CREATE TABLE user (
  id            CHAR(32)     PRIMARY KEY,
  username      VARCHAR(64)  NOT NULL UNIQUE,
  nickname      VARCHAR(64)  NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  role          VARCHAR(16)  NOT NULL DEFAULT 'user',
  is_founder    TINYINT(1)   NOT NULL DEFAULT 0,
  status        VARCHAR(16)  NOT NULL DEFAULT 'active',
  has_avatar    TINYINT(1)   NOT NULL DEFAULT 0,
  bio           TEXT         NOT NULL,
  created_at    DATETIME(3)  NOT NULL,
  updated_at    DATETIME(3)  NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE session (
  id         CHAR(32)    PRIMARY KEY,
  user_id    CHAR(32)    NOT NULL,
  token_hash CHAR(64)    NOT NULL UNIQUE,
  ip         VARCHAR(45) NOT NULL DEFAULT '',
  user_agent TEXT        NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  INDEX idx_session_user (user_id),
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE `blob` (
  sha256     CHAR(64)        PRIMARY KEY,
  kind       VARCHAR(16)     NOT NULL,
  size       BIGINT UNSIGNED NOT NULL,
  ref_count  INT UNSIGNED    NOT NULL DEFAULT 1,
  created_at DATETIME(3)     NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE setting (
  `key` VARCHAR(64) PRIMARY KEY,
  value TEXT        NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
