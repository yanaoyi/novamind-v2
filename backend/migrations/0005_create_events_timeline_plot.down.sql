-- 回滚 0005_create_events_timeline_plot
DROP INDEX IF EXISTS idx_plot_arcs_work;
DROP TABLE IF EXISTS plot_arcs;
DROP INDEX IF EXISTS idx_timeline_events_order;
DROP INDEX IF EXISTS uq_timeline_events;
DROP TABLE IF EXISTS timeline_events;
DROP INDEX IF EXISTS uq_original_timelines_work;
DROP TABLE IF EXISTS original_timelines;
DROP INDEX IF EXISTS idx_original_events_work;
DROP TABLE IF EXISTS original_events;
