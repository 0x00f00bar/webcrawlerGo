CREATE TABLE IF NOT EXISTS urls (
  id bigserial PRIMARY KEY,
  url citext UNIQUE NOT NULL,
  first_encountered timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  last_checked timestamp(0) with time zone DEFAULT NULL,
  last_saved timestamp(0) with time zone DEFAULT NULL,
  is_monitored BOOLEAN NOT NULL DEFAULT false,
  version integer NOT NULL DEFAULT 1 CHECK (version >= 0)
);
