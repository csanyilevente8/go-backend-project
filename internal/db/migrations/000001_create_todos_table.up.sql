CREATE TABLE IF NOT EXISTS todos (
    id          UUID         NOT NULL,
    title       VARCHAR(255) NOT NULL,
    description VARCHAR(2000),
    completed   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP    NOT NULL,
    updated_at  TIMESTAMP    NOT NULL,
    CONSTRAINT pk_todos PRIMARY KEY (id)
);
