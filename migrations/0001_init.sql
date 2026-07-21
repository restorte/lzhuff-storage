CREATE TABLE files
(
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name    text NOT NULL,
    status  text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'done', 'error')),
    size_original   bigint NOT NULL,
    size_compressed bigint NULL,
    sha256  bytea NOT NULL,
    error   text NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_files_pending ON files (created_at) WHERE status = 'pending';
