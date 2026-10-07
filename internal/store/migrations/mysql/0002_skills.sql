-- M2 schema: skill / skill_version / tag / skill_tag (design §6.2, MySQL dialect per §6.1)
-- NOTE: JSON columns are native JSON per §6.1; `blob` stays backticked (BLOB keyword).

CREATE TABLE skill (
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

CREATE TABLE skill_version (
  id            CHAR(32)       PRIMARY KEY,
  skill_id      CHAR(32)       NOT NULL,
  version       VARCHAR(32)    NOT NULL,
  description   TEXT           NOT NULL,
  license       VARCHAR(255)   NOT NULL DEFAULT '',
  compatibility VARCHAR(500)   NOT NULL DEFAULT '',
  author        VARCHAR(255)   NOT NULL DEFAULT '',
  harnesses     JSON           NOT NULL,
  permissions   JSON           NOT NULL,
  frontmatter   JSON           NOT NULL,
  file_count    INT UNSIGNED   NOT NULL DEFAULT 0,
  sha256        CHAR(64)       NOT NULL,
  size          BIGINT UNSIGNED NOT NULL,
  filename      VARCHAR(255)   NOT NULL,
  created_at    DATETIME(3)    NOT NULL,
  UNIQUE (skill_id, version),
  FOREIGN KEY (skill_id) REFERENCES skill(id) ON DELETE CASCADE,
  FOREIGN KEY (sha256) REFERENCES `blob`(sha256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE tag (
  id   CHAR(32)    PRIMARY KEY,
  name VARCHAR(64) NOT NULL UNIQUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE skill_tag (
  skill_id CHAR(32) NOT NULL,
  tag_id   CHAR(32) NOT NULL,
  PRIMARY KEY (skill_id, tag_id),
  FOREIGN KEY (skill_id) REFERENCES skill(id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tag(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
