-- Existing previews remain the default group. Theme labels are independent tags.
ALTER TABLE designmd_version ADD COLUMN preview_light_desktop_sha256 CHAR(64) REFERENCES blob(sha256);
ALTER TABLE designmd_version ADD COLUMN preview_light_mobile_sha256 CHAR(64) REFERENCES blob(sha256);
ALTER TABLE designmd_version ADD COLUMN preview_dark_desktop_sha256 CHAR(64) REFERENCES blob(sha256);
ALTER TABLE designmd_version ADD COLUMN preview_dark_mobile_sha256 CHAR(64) REFERENCES blob(sha256);
