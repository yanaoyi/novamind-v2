-- 0007_create_tasks
-- 异步任务（规格书 §53）：所有长任务异步执行，可查看进度、可重试、可取消。
-- 实现方式：PostgreSQL 做队列（UPDATE ... WHERE id = (SELECT ... FOR UPDATE SKIP LOCKED)），
-- 好处是任务状态与业务数据同库同事务，重启不丢任务；将来要换 Asynq 也只需替换 worker 层。

CREATE TABLE IF NOT EXISTS tasks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID REFERENCES projects(id),
    work_id          UUID REFERENCES original_works(id),
    type             VARCHAR(60)  NOT NULL,
    status           VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    progress         SMALLINT     NOT NULL DEFAULT 0,
    progress_message VARCHAR(200) NOT NULL DEFAULT '',
    input            JSONB        NOT NULL DEFAULT '{}'::jsonb,
    output           JSONB        NOT NULL DEFAULT '{}'::jsonb,
    error            TEXT         NOT NULL DEFAULT '',
    attempts         SMALLINT     NOT NULL DEFAULT 0,
    max_attempts     SMALLINT     NOT NULL DEFAULT 3,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    started_at       TIMESTAMPTZ,
    finished_at      TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT tasks_type_not_blank CHECK (length(btrim(type)) > 0),
    CONSTRAINT tasks_status_valid CHECK (
        status IN ('PENDING', 'RUNNING', 'PAUSED', 'COMPLETED', 'FAILED', 'CANCELLED')
    ),
    CONSTRAINT tasks_progress_range CHECK (progress BETWEEN 0 AND 100),
    CONSTRAINT tasks_attempts_range CHECK (attempts >= 0 AND max_attempts > 0)
);

-- worker 领取任务用
CREATE INDEX IF NOT EXISTS idx_tasks_claim
    ON tasks (status, created_at ASC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_project
    ON tasks (project_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_work
    ON tasks (work_id, created_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE tasks IS '异步任务队列与执行记录；status=PENDING 的行会被 worker 领取执行';
