-- 0017_mappings_unique
-- 映射去重（代码审查 P1-3）：重复继承同一个人物会不断 INSERT 新映射行，
-- 表里于是堆着一模一样的"原著人物 X → 二创人物 Y"。这里先清历史重复，再建唯一索引，
-- 之后仓储层的写入改为 upsert（同一对关系只保留一行）。

-- 1) 清历史重复：同一 (作品, 原著类型, 原著 ID, 二创 ID) 只留最早一行
DELETE FROM original_creative_mappings a
USING original_creative_mappings b
WHERE a.deleted_at IS NULL
  AND b.deleted_at IS NULL
  AND a.id > b.id
  AND a.creative_work_id = b.creative_work_id
  AND a.original_type = b.original_type
  AND a.original_id = b.original_id
  AND a.creative_id = b.creative_id;

-- 2) 唯一索引（部分索引：只约束未删除的行，与代码里的 upsert 目标一致）
CREATE UNIQUE INDEX IF NOT EXISTS uq_mappings_pair
    ON original_creative_mappings (creative_work_id, original_type, original_id, creative_id)
    WHERE deleted_at IS NULL;
