ALTER TABLE designmd_version
  ADD COLUMN preview_light_desktop_sha256 CHAR(64),
  ADD COLUMN preview_light_mobile_sha256 CHAR(64),
  ADD COLUMN preview_dark_desktop_sha256 CHAR(64),
  ADD COLUMN preview_dark_mobile_sha256 CHAR(64),
  ADD FOREIGN KEY (preview_light_desktop_sha256) REFERENCES `blob`(sha256),
  ADD FOREIGN KEY (preview_light_mobile_sha256) REFERENCES `blob`(sha256),
  ADD FOREIGN KEY (preview_dark_desktop_sha256) REFERENCES `blob`(sha256),
  ADD FOREIGN KEY (preview_dark_mobile_sha256) REFERENCES `blob`(sha256);
