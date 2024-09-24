CREATE TABLE IF NOT EXISTS pages(
    id bigserial PRIMARY KEY,
    url_id bigint NOT NULL REFERENCES urls ON DELETE CASCADE,
    added_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    content text NOT NULL
)