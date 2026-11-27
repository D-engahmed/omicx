-- ============================================================
-- Migration 004: Organization Ownership Constraints
-- ============================================================
--
-- Business rule:
--   Each BPO organization can have at most one current owner.
--
-- A current membership means:
--   status IN ('active', 'suspended')
--
-- The organization owner is identified by:
--   roles.code = 'org_owner'
--   roles.scope = 'organization'
--
-- The role ID is resolved dynamically because role IDs are
-- generated using gen_random_uuid().
--
-- Important:
--   This enforces AT MOST one current owner.
--   It does not guarantee that every organization has an owner.
--   Organization creation and ownership transfer must enforce
--   that requirement in application transactions.
-- ============================================================


-- ------------------------------------------------------------
-- 1. Enforce one current owner per organization
-- ------------------------------------------------------------

DO $$
DECLARE
    v_owner_role_id UUID;
BEGIN

    -- Resolve the globally defined organization-owner role.
    SELECT r.id
    INTO STRICT v_owner_role_id
    FROM omicx.roles AS r
    WHERE r.code = 'org_owner'
      AND r.scope = 'organization';

    -- Create a partial unique index.
    --
    -- This prevents an organization from having two memberships
    -- assigned the org_owner role while those memberships are
    -- active or suspended.
    --
    -- Ended memberships are historical and do not count.

    EXECUTE format(
        'CREATE UNIQUE INDEX organization_memberships_one_current_owner_per_org
         ON omicx.organization_memberships (organization_id)
         WHERE status IN (''active'', ''suspended'')
           AND organization_role_scope = ''organization''
           AND organization_role_id = %L::uuid',
        v_owner_role_id
    );

END;
$$;


COMMENT ON INDEX
    omicx.organization_memberships_one_current_owner_per_org
IS
    'Ensures that each BPO organization has at most one current owner membership.';

---- create above / drop below ----

-- ============================================================
-- Rollback Migration 004
-- ============================================================
-- Remove the current-owner uniqueness constraint.
-- Existing organization memberships remain unchanged.
-- ============================================================

DROP INDEX omicx.organization_memberships_one_current_owner_per_org;