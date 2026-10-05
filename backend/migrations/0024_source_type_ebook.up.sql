-- 0024_source_type_ebook
-- 支持电子书格式导入（EPUB / MOBI）。
--
-- 背景：正文抽取已实现（internal/parser/epub.go、mobi.go），但 original_works.source_type
-- 的 CHECK 约束是 0002 里定死的四类，不放开的话导入会在写入来源类型时违反约束。
--
-- 做法：先删旧约束再以新清单重建（IF EXISTS 保证可重复执行）。

ALTER TABLE original_works DROP CONSTRAINT IF EXISTS original_works_source_type_valid;

ALTER TABLE original_works
    ADD CONSTRAINT original_works_source_type_valid
    CHECK (source_type IN ('MANUAL', 'TXT', 'DOCX', 'PDF', 'EPUB', 'MOBI'));

COMMENT ON COLUMN original_works.source_type IS 'MANUAL | TXT | DOCX | PDF | EPUB | MOBI';
