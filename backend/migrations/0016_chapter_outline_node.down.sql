-- 0016_chapter_outline_node (down)

DROP INDEX IF EXISTS uq_creative_chapters_outline_node;
ALTER TABLE creative_chapters DROP COLUMN IF EXISTS outline_node_id;
