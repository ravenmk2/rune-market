-- M4: images keep their original format (ext column) and skills gain an
-- optional icon referencing an image blob (design §6.1, §11)

ALTER TABLE `blob` ADD COLUMN ext VARCHAR(8) NOT NULL DEFAULT '';
UPDATE `blob` SET ext = 'png' WHERE kind = 'image';

ALTER TABLE skill
  ADD COLUMN icon_sha256 CHAR(64) NULL,
  ADD CONSTRAINT fk_skill_icon FOREIGN KEY (icon_sha256) REFERENCES `blob`(sha256);
