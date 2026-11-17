
-- Create the application-owned schema.
CREATE SCHEMA omicx;

COMMENT ON SCHEMA omicx IS
    'Application data schema for the OMICX platform.';

---- create above / drop below ----

-- Rollback the migration.
-- No CASCADE: refuse to remove the schema if it contains objects.
DROP SCHEMA omicx;
