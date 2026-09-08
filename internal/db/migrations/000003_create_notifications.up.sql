CREATE TABLE IF NOT EXISTS notifications (
    id          BIGSERIAL    PRIMARY KEY,
    todo_id     UUID         NOT NULL,
    type        VARCHAR(32)  NOT NULL,
    message     VARCHAR(512) NOT NULL,
    read        BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMP    NOT NULL DEFAULT now()
);
