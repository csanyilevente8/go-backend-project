CREATE TABLE IF NOT EXISTS activity_log (
    id          BIGSERIAL    PRIMARY KEY,
    todo_id     UUID         NOT NULL,
    type        VARCHAR(32)  NOT NULL,
    detail      VARCHAR(512),
    created_at  TIMESTAMP    NOT NULL DEFAULT now()
);
