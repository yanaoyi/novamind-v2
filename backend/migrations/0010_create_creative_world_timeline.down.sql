-- 回滚 0010_create_creative_world_timeline
DROP INDEX IF EXISTS uq_creative_timeline_source;
DROP INDEX IF EXISTS idx_creative_timeline_events_order;
DROP TABLE IF EXISTS creative_timeline_events;
DROP INDEX IF EXISTS uq_divergence_points_work;
DROP TABLE IF EXISTS divergence_points;
DROP INDEX IF EXISTS idx_creative_world_rules_world;
DROP INDEX IF EXISTS uq_creative_world_rules_name;
DROP TABLE IF EXISTS creative_world_rules;
DROP INDEX IF EXISTS uq_creative_worlds_work;
DROP TABLE IF EXISTS creative_worlds;
