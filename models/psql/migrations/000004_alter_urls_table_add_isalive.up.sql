ALTER TABLE urls
ADD COLUMN IF NOT EXISTS is_alive BOOLEAN DEFAULT TRUE;
