-- 0024_source_type_ebook (down)
-- 回滚前先把电子书来源的记录收敛成 TXT，否则旧约束会拒绝。

UPDATE original_works SET source_type = 'TXT' WHERE source_type IN ('EPUB', 'MOBI');

ALTER TABLE original_works DROP CONSTRAINT IF EXISTS original_works_source_type_valid;

ALTER TABLE original_works
    ADD CONSTRAINT original_works_source_type_valid
    CHECK (source_type IN ('MANUAL', 'TXT', 'DOCX', 'PDF'));
