ALTER TABLE skill ADD COLUMN display_name VARCHAR(64) NOT NULL DEFAULT '';
UPDATE skill SET display_name = name;
