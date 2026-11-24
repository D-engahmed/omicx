-- Migration 003: RBAC catalog and membership-role constraints
--
-- Design:
--   * Roles and permissions are global, predefined catalog records in v1.
--   * Every permission has an explicit scope: organization or team.
--   * A role can only receive permissions from the same scope.
--   * Every organization membership must have an organization-scoped role.
--   * Team membership already requires a team-scoped role (Migration 002).
--
-- Custom tenant-defined roles are intentionally deferred. The current roles
-- table has no organization_id, so is_system = false does not create tenant
-- ownership or isolation.

-- ------------------------------------------------------------
-- 1. Add explicit scope to permissions
-- ------------------------------------------------------------

ALTER TABLE omicx.permissions
    ADD COLUMN scope TEXT;

-- Classify any pre-existing permission codes. Unknown prefixes are rejected
-- below instead of being silently assigned to an arbitrary scope.
UPDATE omicx.permissions
SET scope = CASE
    WHEN code LIKE 'organization.%' THEN 'organization'
    WHEN code LIKE 'team.%' THEN 'team'
    ELSE NULL
END
WHERE scope IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM omicx.permissions
        WHERE scope IS NULL
    ) THEN
        RAISE EXCEPTION
            'Migration 003: permissions contains codes that do not start with organization. or team.; classify them before migrating';
    END IF;
END;
$$;

ALTER TABLE omicx.permissions
    ALTER COLUMN scope SET NOT NULL,
    ADD CONSTRAINT permissions_scope_check
        CHECK (scope IN ('organization', 'team')),
    ADD CONSTRAINT permissions_code_scope_check
        CHECK (
            (scope = 'organization' AND code LIKE 'organization.%')
            OR
            (scope = 'team' AND code LIKE 'team.%')
        ),
    ADD CONSTRAINT permissions_id_scope_unique
        UNIQUE (id, scope);

-- ------------------------------------------------------------
-- 2. Enforce that role-permission mappings have matching scopes
-- ------------------------------------------------------------

ALTER TABLE omicx.role_permissions
    ADD COLUMN scope TEXT;

UPDATE omicx.role_permissions AS rp
SET scope = r.scope
FROM omicx.roles AS r
WHERE r.id = rp.role_id
  AND rp.scope IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM omicx.role_permissions AS rp
        LEFT JOIN omicx.roles AS r ON r.id = rp.role_id
        LEFT JOIN omicx.permissions AS p ON p.id = rp.permission_id
        WHERE rp.scope IS NULL
           OR r.id IS NULL
           OR p.id IS NULL
           OR r.scope <> p.scope
           OR rp.scope <> r.scope
    ) THEN
        RAISE EXCEPTION
            'Migration 003: role_permissions contains a role/permission scope mismatch; resolve mappings before migrating';
    END IF;
END;
$$;

ALTER TABLE omicx.role_permissions
    ALTER COLUMN scope SET NOT NULL,
    ADD CONSTRAINT role_permissions_scope_check
        CHECK (scope IN ('organization', 'team'));

-- Replace single-column foreign keys with composite scope-aware foreign keys.
ALTER TABLE omicx.role_permissions
    DROP CONSTRAINT role_permissions_role_fk,
    DROP CONSTRAINT role_permissions_permission_fk,
    ADD CONSTRAINT role_permissions_role_scope_fk
        FOREIGN KEY (role_id, scope)
        REFERENCES omicx.roles (id, scope),
    ADD CONSTRAINT role_permissions_permission_scope_fk
        FOREIGN KEY (permission_id, scope)
        REFERENCES omicx.permissions (id, scope);

COMMENT ON COLUMN omicx.permissions.scope IS
    'Authorization context for this permission: organization or team.';

COMMENT ON COLUMN omicx.role_permissions.scope IS
    'Shared scope enforced against both the referenced role and permission.';

-- ------------------------------------------------------------
-- 3. Seed predefined global roles
-- ------------------------------------------------------------

INSERT INTO omicx.roles (code, name, description, scope, is_system)
VALUES
    ('org_owner', 'Organization Owner',
     'Owns the BPO organization and can transfer ownership.',
     'organization', TRUE),
    ('org_admin', 'Organization Administrator',
     'Administers BPO organization settings, membership, clients, and teams.',
     'organization', TRUE),
    ('org_manager', 'Organization Manager',
     'Manages client companies, teams, team assignments, and operational oversight.',
     'organization', TRUE),
    ('org_member', 'Organization Member',
     'Basic BPO organization membership without administrative privileges.',
     'organization', TRUE),
    ('team_lead', 'Team Lead',
     'Leads a team and manages its members and operational work.',
     'team', TRUE),
    ('team_agent', 'Team Agent',
     'Handles customer-support work within an assigned team.',
     'team', TRUE),
    ('team_viewer', 'Team Viewer',
     'Has read-only access to permitted team information.',
     'team', TRUE)
ON CONFLICT (scope, code) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    is_system = TRUE;

-- ------------------------------------------------------------
-- 4. Seed the initial permission catalog
-- ------------------------------------------------------------

INSERT INTO omicx.permissions (code, description, scope)
VALUES
    -- Organization permissions
    ('organization.read',
     'Read basic BPO organization information.', 'organization'),
    ('organization.settings.update',
     'Update BPO organization settings.', 'organization'),
    ('organization.members.read',
     'Read BPO organization membership records.', 'organization'),
    ('organization.members.add',
     'Add an existing user to the BPO organization.', 'organization'),
    ('organization.members.update_role',
     'Change an organization member role, subject to protected-role rules.', 'organization'),
    ('organization.members.suspend',
     'Suspend an organization membership.', 'organization'),
    ('organization.members.terminate',
     'Terminate an organization membership.', 'organization'),
    ('organization.clients.read',
     'Read client companies belonging to the BPO organization.', 'organization'),
    ('organization.clients.create',
     'Create a client company for the BPO organization.', 'organization'),
    ('organization.clients.update',
     'Update client company records.', 'organization'),
    ('organization.teams.read',
     'Read operational teams belonging to the BPO organization.', 'organization'),
    ('organization.teams.create',
     'Create operational teams.', 'organization'),
    ('organization.teams.update',
     'Update operational team records.', 'organization'),
    ('organization.team_memberships.manage',
     'Assign or remove organization members from teams.', 'organization'),
    ('organization.team_client_assignments.manage',
     'Assign or remove teams from client companies.', 'organization'),
    ('organization.conversations.read_all',
     'Read conversations across all teams in the BPO organization.', 'organization'),
    ('organization.ownership.transfer',
     'Transfer ownership of the BPO organization.', 'organization'),

    -- Team permissions
    ('team.read',
     'Read information about the current team.', 'team'),
    ('team.members.read',
     'Read membership records for the current team.', 'team'),
    ('team.members.manage',
     'Manage membership of existing organization members in the current team.', 'team'),
    ('team.conversations.read',
     'Read conversations accessible to the current team.', 'team'),
    ('team.conversations.reply',
     'Send replies to conversations accessible to the current team.', 'team'),
    ('team.conversations.assign',
     'Assign conversations within the current team.', 'team'),
    ('team.contacts.read',
     'Read customer contact information available to the current team.', 'team'),
    ('team.knowledge.read',
     'Read knowledge resources available to the current team.', 'team')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description,
    scope = EXCLUDED.scope;

-- ------------------------------------------------------------
-- 5. Seed role-permission mappings
-- ------------------------------------------------------------
-- Organization roles grant organization-scoped permissions only. Team roles
-- grant team-scoped permissions only. Organization-wide conversation reading
-- is an explicit organization permission; it is not implied by team membership.

WITH grants (role_code, scope, permission_code) AS (
    VALUES
        -- org_owner: all organization permissions, including ownership transfer
        ('org_owner', 'organization', 'organization.read'),
        ('org_owner', 'organization', 'organization.settings.update'),
        ('org_owner', 'organization', 'organization.members.read'),
        ('org_owner', 'organization', 'organization.members.add'),
        ('org_owner', 'organization', 'organization.members.update_role'),
        ('org_owner', 'organization', 'organization.members.suspend'),
        ('org_owner', 'organization', 'organization.members.terminate'),
        ('org_owner', 'organization', 'organization.clients.read'),
        ('org_owner', 'organization', 'organization.clients.create'),
        ('org_owner', 'organization', 'organization.clients.update'),
        ('org_owner', 'organization', 'organization.teams.read'),
        ('org_owner', 'organization', 'organization.teams.create'),
        ('org_owner', 'organization', 'organization.teams.update'),
        ('org_owner', 'organization', 'organization.team_memberships.manage'),
        ('org_owner', 'organization', 'organization.team_client_assignments.manage'),
        ('org_owner', 'organization', 'organization.conversations.read_all'),
        ('org_owner', 'organization', 'organization.ownership.transfer'),

        -- org_admin: all organization permissions except ownership transfer
        ('org_admin', 'organization', 'organization.read'),
        ('org_admin', 'organization', 'organization.settings.update'),
        ('org_admin', 'organization', 'organization.members.read'),
        ('org_admin', 'organization', 'organization.members.add'),
        ('org_admin', 'organization', 'organization.members.update_role'),
        ('org_admin', 'organization', 'organization.members.suspend'),
        ('org_admin', 'organization', 'organization.members.terminate'),
        ('org_admin', 'organization', 'organization.clients.read'),
        ('org_admin', 'organization', 'organization.clients.create'),
        ('org_admin', 'organization', 'organization.clients.update'),
        ('org_admin', 'organization', 'organization.teams.read'),
        ('org_admin', 'organization', 'organization.teams.create'),
        ('org_admin', 'organization', 'organization.teams.update'),
        ('org_admin', 'organization', 'organization.team_memberships.manage'),
        ('org_admin', 'organization', 'organization.team_client_assignments.manage'),
        ('org_admin', 'organization', 'organization.conversations.read_all'),

        -- org_manager: operational administration, not identity/role administration
        ('org_manager', 'organization', 'organization.read'),
        ('org_manager', 'organization', 'organization.members.read'),
        ('org_manager', 'organization', 'organization.clients.read'),
        ('org_manager', 'organization', 'organization.clients.create'),
        ('org_manager', 'organization', 'organization.clients.update'),
        ('org_manager', 'organization', 'organization.teams.read'),
        ('org_manager', 'organization', 'organization.teams.create'),
        ('org_manager', 'organization', 'organization.teams.update'),
        ('org_manager', 'organization', 'organization.team_memberships.manage'),
        ('org_manager', 'organization', 'organization.team_client_assignments.manage'),

        -- org_member: basic organization visibility only
        ('org_member', 'organization', 'organization.read'),

        -- team_lead: team operations and member supervision
        ('team_lead', 'team', 'team.read'),
        ('team_lead', 'team', 'team.members.read'),
        ('team_lead', 'team', 'team.members.manage'),
        ('team_lead', 'team', 'team.conversations.read'),
        ('team_lead', 'team', 'team.conversations.reply'),
        ('team_lead', 'team', 'team.conversations.assign'),
        ('team_lead', 'team', 'team.contacts.read'),
        ('team_lead', 'team', 'team.knowledge.read'),

        -- team_agent: support work, without team administration
        ('team_agent', 'team', 'team.read'),
        ('team_agent', 'team', 'team.conversations.read'),
        ('team_agent', 'team', 'team.conversations.reply'),
        ('team_agent', 'team', 'team.contacts.read'),
        ('team_agent', 'team', 'team.knowledge.read'),

        -- team_viewer: read-only team access
        ('team_viewer', 'team', 'team.read'),
        ('team_viewer', 'team', 'team.members.read'),
        ('team_viewer', 'team', 'team.conversations.read'),
        ('team_viewer', 'team', 'team.contacts.read'),
        ('team_viewer', 'team', 'team.knowledge.read')
)
INSERT INTO omicx.role_permissions (role_id, permission_id, scope)
SELECT r.id, p.id, r.scope
FROM grants AS g
JOIN omicx.roles AS r
  ON r.code = g.role_code
 AND r.scope = g.scope
JOIN omicx.permissions AS p
  ON p.code = g.permission_code
 AND p.scope = g.scope
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ------------------------------------------------------------
-- 6. Ensure every organization membership has an organization role
-- ------------------------------------------------------------

UPDATE omicx.organization_memberships AS m
SET organization_role_id = r.id
FROM omicx.roles AS r
WHERE m.organization_role_id IS NULL
  AND r.scope = 'organization'
  AND r.code = 'org_member';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM omicx.organization_memberships
        WHERE organization_role_id IS NULL
    ) THEN
        RAISE EXCEPTION
            'Migration 003: one or more organization memberships could not be assigned the org_member baseline role';
    END IF;
END;
$$;

ALTER TABLE omicx.organization_memberships
    ALTER COLUMN organization_role_id SET NOT NULL;

COMMENT ON COLUMN omicx.organization_memberships.organization_role_id IS
    'Required organization-scoped role assigned to this BPO membership.';

-- End of Migration 003.
