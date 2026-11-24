-- ============================================================
-- Migration 002: Identity and BPO Core
-- ============================================================
-- Creates:
--   * Global user identities
--   * BPO organizations (tenants) and historical memberships
--   * Global role and permission definitions
--   * Password credentials and refresh-token sessions
--   * Client companies served by each BPO
--   * BPO teams, team memberships, and team/client assignments
--
-- Business rules:
--   * One organization represents one BPO tenant.
--   * A BPO can operate multiple client companies.
--   * A user can have at most one current BPO membership globally.
--   * Ended memberships are retained as history; rejoining creates
--     a new membership row.
--   * A team and a client company must belong to the same BPO.
--   * Teams can serve multiple client companies, and a client company
--     can be served by multiple teams.
--
-- Out of scope for this migration:
--   * Invitations and invitation tokens
--   * Client-company channels and integrations
--   * End customers, conversations, and messages
--   * RLS policies and application authorization implementation
-- ============================================================

-- Least-privilege baseline for tables created by this migrator.
-- Runtime permissions are granted explicitly below; the API role
-- does not receive automatic DELETE or sequence privileges.
ALTER DEFAULT PRIVILEGES
FOR ROLE omicx_migrator
IN SCHEMA omicx
REVOKE ALL ON TABLES FROM omicx_app;

ALTER DEFAULT PRIVILEGES
FOR ROLE omicx_migrator
IN SCHEMA omicx
REVOKE ALL ON SEQUENCES FROM omicx_app;


-- ------------------------------------------------------------
-- 1. Global users
-- ------------------------------------------------------------

CREATE TABLE omicx.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    email TEXT NOT NULL,
    display_name TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'active',
    email_verified_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_email_normalized_check
        CHECK (email = LOWER(BTRIM(email)) AND email <> ''),

    CONSTRAINT users_display_name_check
        CHECK (BTRIM(display_name) <> ''),

    CONSTRAINT users_status_check
        CHECK (status IN ('active', 'suspended', 'disabled')),

    CONSTRAINT users_email_unique UNIQUE (email)
);

COMMENT ON TABLE omicx.users IS
    'Global human identities across the OMICX platform.';


-- ------------------------------------------------------------
-- 2. BPO organizations (tenants)
-- ------------------------------------------------------------

CREATE TABLE omicx.organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name TEXT NOT NULL,
    slug TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'active',
    default_timezone TEXT NOT NULL DEFAULT 'UTC',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT organizations_name_check
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT organizations_slug_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),

    CONSTRAINT organizations_status_check
        CHECK (status IN ('active', 'suspended', 'closed')),

    CONSTRAINT organizations_slug_unique UNIQUE (slug)
);

COMMENT ON TABLE omicx.organizations IS
    'BPO companies subscribing to and operating the OMICX platform.';


-- ------------------------------------------------------------
-- 3. Global role definitions
-- ------------------------------------------------------------

CREATE TABLE omicx.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    scope TEXT NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT roles_scope_check
        CHECK (scope IN ('organization', 'team')),

    CONSTRAINT roles_code_check
        CHECK (code ~ '^[a-z][a-z0-9_]*$'),

    CONSTRAINT roles_name_check
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT roles_scope_code_unique
        UNIQUE (scope, code),

    CONSTRAINT roles_id_scope_unique
        UNIQUE (id, scope)
);

COMMENT ON TABLE omicx.roles IS
    'Global role definitions. Role assignments are scoped to a BPO or team.';


-- ------------------------------------------------------------
-- 4. Global permissions
-- ------------------------------------------------------------

CREATE TABLE omicx.permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    code TEXT NOT NULL,
    description TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT permissions_code_unique UNIQUE (code),

    CONSTRAINT permissions_code_check
        CHECK (code ~ '^[a-z][a-z0-9_]*([.][a-z][a-z0-9_]*)+$')
);

COMMENT ON TABLE omicx.permissions IS
    'Global permission catalogue used by role-based authorization.';


-- ------------------------------------------------------------
-- 5. Role-to-permission relationships
-- ------------------------------------------------------------

CREATE TABLE omicx.role_permissions (
    role_id UUID NOT NULL,
    permission_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT role_permissions_pkey
        PRIMARY KEY (role_id, permission_id),

    CONSTRAINT role_permissions_role_fk
        FOREIGN KEY (role_id)
        REFERENCES omicx.roles (id),

    CONSTRAINT role_permissions_permission_fk
        FOREIGN KEY (permission_id)
        REFERENCES omicx.permissions (id)
);

-- Supports permission-to-roles lookups and permission deletion checks.
CREATE INDEX role_permissions_permission_idx
ON omicx.role_permissions (permission_id);


-- ------------------------------------------------------------
-- 6. BPO organization memberships
-- ------------------------------------------------------------

CREATE TABLE omicx.organization_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,
    user_id UUID NOT NULL,

    -- A membership may have one BPO-level role.
    -- Team-specific roles are assigned in team_memberships.
    -- NULL is permitted when a person has no BPO-wide role.
    organization_role_id UUID,
    organization_role_scope TEXT NOT NULL DEFAULT 'organization',

    status TEXT NOT NULL DEFAULT 'active',

    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT organization_memberships_organization_fk
        FOREIGN KEY (organization_id)
        REFERENCES omicx.organizations (id),

    CONSTRAINT organization_memberships_user_fk
        FOREIGN KEY (user_id)
        REFERENCES omicx.users (id),

    CONSTRAINT organization_memberships_role_fk
        FOREIGN KEY (organization_role_id, organization_role_scope)
        REFERENCES omicx.roles (id, scope),

    CONSTRAINT organization_memberships_role_scope_check
        CHECK (organization_role_scope = 'organization'),

    CONSTRAINT organization_memberships_status_check
        CHECK (status IN ('active', 'suspended', 'ended')),

    CONSTRAINT organization_memberships_dates_check
        CHECK (
            (status IN ('active', 'suspended') AND ended_at IS NULL)
            OR
            (status = 'ended' AND ended_at IS NOT NULL)
        ),

    CONSTRAINT organization_memberships_date_order_check
        CHECK (ended_at IS NULL OR ended_at >= started_at),

    -- Required for tenant-aware composite foreign keys.
    CONSTRAINT organization_memberships_org_id_unique
        UNIQUE (organization_id, id)
);

COMMENT ON TABLE omicx.organization_memberships IS
    'Current and historical affiliation of users with BPO tenants.';

-- One user may have at most one current BPO membership globally.
-- Suspended memberships still count as current. Ended rows are history.
CREATE UNIQUE INDEX organization_memberships_one_current_per_user
ON omicx.organization_memberships (user_id)
WHERE status IN ('active', 'suspended');

CREATE INDEX organization_memberships_org_status_idx
ON omicx.organization_memberships (organization_id, status);

CREATE INDEX organization_memberships_user_history_idx
ON omicx.organization_memberships (user_id, started_at DESC);

CREATE INDEX organization_memberships_role_idx
ON omicx.organization_memberships (organization_role_id)
WHERE organization_role_id IS NOT NULL;


-- ------------------------------------------------------------
-- 7. Password credentials
-- ------------------------------------------------------------

CREATE TABLE omicx.password_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,
    password_hash TEXT NOT NULL,
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT password_credentials_user_unique UNIQUE (user_id),

    CONSTRAINT password_credentials_user_fk
        FOREIGN KEY (user_id)
        REFERENCES omicx.users (id)
        ON DELETE CASCADE,

    CONSTRAINT password_credentials_hash_check
        CHECK (BTRIM(password_hash) <> '')
);

COMMENT ON TABLE omicx.password_credentials IS
    'Password hashes for global users. Plaintext passwords must never be stored.';


-- ------------------------------------------------------------
-- 8. User sessions
-- ------------------------------------------------------------

CREATE TABLE omicx.user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    -- SHA-256 digest of a cryptographically random refresh token.
    -- Store the 32-byte digest, never the raw token.
    refresh_token_hash BYTEA NOT NULL,
    token_family_id UUID NOT NULL,

    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_sessions_refresh_token_hash_unique
        UNIQUE (refresh_token_hash),

    CONSTRAINT user_sessions_user_fk
        FOREIGN KEY (user_id)
        REFERENCES omicx.users (id)
        ON DELETE CASCADE,

    CONSTRAINT user_sessions_hash_length_check
        CHECK (OCTET_LENGTH(refresh_token_hash) = 32),

    CONSTRAINT user_sessions_expiry_check
        CHECK (expires_at > created_at),

    CONSTRAINT user_sessions_last_used_check
        CHECK (last_used_at IS NULL OR last_used_at >= created_at),

    CONSTRAINT user_sessions_revoked_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX user_sessions_user_idx
ON omicx.user_sessions (user_id);

CREATE INDEX user_sessions_token_family_idx
ON omicx.user_sessions (token_family_id);

CREATE INDEX user_sessions_active_expiry_idx
ON omicx.user_sessions (expires_at)
WHERE revoked_at IS NULL;

COMMENT ON TABLE omicx.user_sessions IS
    'Refresh-token sessions. Only SHA-256 token digests are stored.';


-- ------------------------------------------------------------
-- 9. Client companies served by a BPO
-- ------------------------------------------------------------

CREATE TABLE omicx.client_companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,

    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    legal_name TEXT,

    country_code CHAR(2),
    currency_code CHAR(3),

    default_language TEXT NOT NULL DEFAULT 'en',
    default_timezone TEXT NOT NULL DEFAULT 'UTC',

    status TEXT NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT client_companies_organization_fk
        FOREIGN KEY (organization_id)
        REFERENCES omicx.organizations (id),

    CONSTRAINT client_companies_name_check
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT client_companies_slug_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),

    CONSTRAINT client_companies_country_check
        CHECK (country_code IS NULL OR country_code ~ '^[A-Z]{2}$'),

    CONSTRAINT client_companies_currency_check
        CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$'),

    CONSTRAINT client_companies_status_check
        CHECK (status IN ('active', 'suspended', 'archived')),

    CONSTRAINT client_companies_org_slug_unique
        UNIQUE (organization_id, slug),

    -- Required for tenant-aware composite foreign keys.
    CONSTRAINT client_companies_org_id_unique
        UNIQUE (organization_id, id)
);

COMMENT ON TABLE omicx.client_companies IS
    'Businesses whose customer support operations are managed by a BPO tenant.';

CREATE INDEX client_companies_org_status_idx
ON omicx.client_companies (organization_id, status);


-- ------------------------------------------------------------
-- 10. BPO operational teams
-- ------------------------------------------------------------

CREATE TABLE omicx.teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,

    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,

    status TEXT NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT teams_organization_fk
        FOREIGN KEY (organization_id)
        REFERENCES omicx.organizations (id),

    CONSTRAINT teams_name_check
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT teams_slug_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),

    CONSTRAINT teams_status_check
        CHECK (status IN ('active', 'archived')),

    CONSTRAINT teams_org_slug_unique
        UNIQUE (organization_id, slug),

    -- Required for tenant-aware composite foreign keys.
    CONSTRAINT teams_org_id_unique
        UNIQUE (organization_id, id)
);

COMMENT ON TABLE omicx.teams IS
    'Operational teams belonging to a BPO organization.';

CREATE INDEX teams_org_status_idx
ON omicx.teams (organization_id, status);


-- ------------------------------------------------------------
-- 11. Team memberships
-- ------------------------------------------------------------

CREATE TABLE omicx.team_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,
    team_id UUID NOT NULL,
    organization_membership_id UUID NOT NULL,

    team_role_id UUID NOT NULL,
    team_role_scope TEXT NOT NULL DEFAULT 'team',

    status TEXT NOT NULL DEFAULT 'active',

    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT team_memberships_team_fk
        FOREIGN KEY (organization_id, team_id)
        REFERENCES omicx.teams (organization_id, id),

    CONSTRAINT team_memberships_organization_membership_fk
        FOREIGN KEY (organization_id, organization_membership_id)
        REFERENCES omicx.organization_memberships (organization_id, id),

    CONSTRAINT team_memberships_role_fk
        FOREIGN KEY (team_role_id, team_role_scope)
        REFERENCES omicx.roles (id, scope),

    CONSTRAINT team_memberships_role_scope_check
        CHECK (team_role_scope = 'team'),

    CONSTRAINT team_memberships_status_check
        CHECK (status IN ('active', 'ended')),

    CONSTRAINT team_memberships_dates_check
        CHECK (
            (status = 'active' AND ended_at IS NULL)
            OR
            (status = 'ended' AND ended_at IS NOT NULL)
        ),

    CONSTRAINT team_memberships_date_order_check
        CHECK (ended_at IS NULL OR ended_at >= started_at),

    CONSTRAINT team_memberships_org_id_unique
        UNIQUE (organization_id, id)
);

-- One employee may belong to several teams, but not twice actively
-- in the same team under the same BPO membership.
CREATE UNIQUE INDEX team_memberships_one_active_per_team_member
ON omicx.team_memberships (team_id, organization_membership_id)
WHERE status = 'active';

CREATE INDEX team_memberships_member_idx
ON omicx.team_memberships (organization_id, organization_membership_id);

CREATE INDEX team_memberships_team_history_idx
ON omicx.team_memberships (organization_id, team_id);

CREATE INDEX team_memberships_role_idx
ON omicx.team_memberships (team_role_id);

COMMENT ON TABLE omicx.team_memberships IS
    'Assignments of BPO organization members to operational teams.';


-- ------------------------------------------------------------
-- 12. Assign teams to client companies
-- ------------------------------------------------------------

CREATE TABLE omicx.team_client_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,
    team_id UUID NOT NULL,
    client_company_id UUID NOT NULL,

    status TEXT NOT NULL DEFAULT 'active',

    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT team_client_assignments_team_fk
        FOREIGN KEY (organization_id, team_id)
        REFERENCES omicx.teams (organization_id, id),

    CONSTRAINT team_client_assignments_client_fk
        FOREIGN KEY (organization_id, client_company_id)
        REFERENCES omicx.client_companies (organization_id, id),

    CONSTRAINT team_client_assignments_status_check
        CHECK (status IN ('active', 'ended')),

    CONSTRAINT team_client_assignments_dates_check
        CHECK (
            (status = 'active' AND ended_at IS NULL)
            OR
            (status = 'ended' AND ended_at IS NOT NULL)
        ),

    CONSTRAINT team_client_assignments_date_order_check
        CHECK (ended_at IS NULL OR ended_at >= assigned_at)
);

-- A team can serve multiple clients and a client can be served
-- by multiple teams. Historical assignments are retained.
CREATE UNIQUE INDEX team_client_assignments_one_active_pair
ON omicx.team_client_assignments (team_id, client_company_id)
WHERE status = 'active';

CREATE INDEX team_client_assignments_client_idx
ON omicx.team_client_assignments (organization_id, client_company_id);

CREATE INDEX team_client_assignments_team_history_idx
ON omicx.team_client_assignments (organization_id, team_id);

COMMENT ON TABLE omicx.team_client_assignments IS
    'Current and historical assignments of BPO teams to client companies.';


-- ------------------------------------------------------------
-- 13. Runtime application privileges
-- ------------------------------------------------------------
-- The API can read and modify operational records, but cannot
-- hard-delete records. Global RBAC definitions are read-only at
-- runtime and are maintained through reviewed migrations.

GRANT SELECT, INSERT, UPDATE ON TABLE
    omicx.users,
    omicx.organizations,
    omicx.organization_memberships,
    omicx.password_credentials,
    omicx.user_sessions,
    omicx.client_companies,
    omicx.teams,
    omicx.team_memberships,
    omicx.team_client_assignments
TO omicx_app;

GRANT SELECT ON TABLE
    omicx.roles,
    omicx.permissions,
    omicx.role_permissions
TO omicx_app;


---- create above / drop below ----


-- ============================================================
-- Rollback migration 002
-- ============================================================
-- Drop dependent tables before their referenced parent tables.
-- ============================================================

DROP TABLE omicx.team_client_assignments;
DROP TABLE omicx.team_memberships;
DROP TABLE omicx.teams;
DROP TABLE omicx.client_companies;

DROP TABLE omicx.user_sessions;
DROP TABLE omicx.password_credentials;

DROP TABLE omicx.organization_memberships;

DROP TABLE omicx.role_permissions;
DROP TABLE omicx.permissions;
DROP TABLE omicx.roles;

DROP TABLE omicx.organizations;
DROP TABLE omicx.users;

-- Restore the default privileges that existed before this migration.
ALTER DEFAULT PRIVILEGES
FOR ROLE omicx_migrator
IN SCHEMA omicx
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO omicx_app;

ALTER DEFAULT PRIVILEGES
FOR ROLE omicx_migrator
IN SCHEMA omicx
GRANT USAGE, SELECT ON SEQUENCES TO omicx_app;
