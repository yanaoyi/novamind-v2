-- 0016_chapter_outline_node
-- 大纲落成章节的可追溯与防重（代码审查 P1-2）。
--
-- 之前的问题：POST /outlines/{id}/materialize 逐条写卷与章节、不在同一事务里，
-- 中途失败会留下半成品；而且重复点一次就再追加一遍同样的章节。
--
-- 这里给章节记上"来自哪个大纲节点"，并对它建部分唯一索引：
--   * 同一个大纲节点只会落成一章（重复落成时跳过，不再重复建）；
--   * 也顺便回答"这一章是从哪条大纲来的"，便于追溯。

ALTER TABLE creative_chapters
    ADD COLUMN IF NOT EXISTS outline_node_id UUID REFERENCES outline_nodes(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_chapters_outline_node
    ON creative_chapters (outline_node_id)
    WHERE outline_node_id IS NOT NULL AND deleted_at IS NULL;

COMMENT ON COLUMN creative_chapters.outline_node_id IS '来源大纲节点（§27 落成章节时写入），用于防止重复落成';
