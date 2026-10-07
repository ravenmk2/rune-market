-- M3 schema: designmd / designmd_version / designmd_tag (design §6.2, MySQL dialect per §6.1)
-- NOTE: content is MEDIUMTEXT per §6.1 (长文本); `blob` stays backticked.

CREATE TABLE designmd (
  id                CHAR(32)       PRIMARY KEY,
  owner_id          CHAR(32)       NOT NULL,
  name              VARCHAR(64)    NOT NULL,
  summary           VARCHAR(1024)  NOT NULL DEFAULT '',
  official          TINYINT(1)     NOT NULL DEFAULT 0,
  status            VARCHAR(16)    NOT NULL DEFAULT 'published',
  latest_version_id CHAR(32),
  download_count    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at        DATETIME(3)    NOT NULL,
  updated_at        DATETIME(3)    NOT NULL,
  UNIQUE (owner_id, name),
  FOREIGN KEY (owner_id) REFERENCES user(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE designmd_version (
  id                     CHAR(32)    PRIMARY KEY,
  designmd_id            CHAR(32)    NOT NULL,
  version                VARCHAR(32) NOT NULL,
  content                MEDIUMTEXT  NOT NULL,
  sha256                 CHAR(64)    NOT NULL,
  preview_desktop_sha256 CHAR(64),
  preview_mobile_sha256  CHAR(64),
  created_at             DATETIME(3) NOT NULL,
  UNIQUE (designmd_id, version),
  FOREIGN KEY (designmd_id) REFERENCES designmd(id) ON DELETE CASCADE,
  FOREIGN KEY (preview_desktop_sha256) REFERENCES `blob`(sha256),
  FOREIGN KEY (preview_mobile_sha256) REFERENCES `blob`(sha256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE designmd_tag (
  designmd_id CHAR(32) NOT NULL,
  tag_id      CHAR(32) NOT NULL,
  PRIMARY KEY (designmd_id, tag_id),
  FOREIGN KEY (designmd_id) REFERENCES designmd(id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tag(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
