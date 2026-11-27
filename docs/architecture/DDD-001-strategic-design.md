# DDD-001 — OMICX Strategic Design & Bounded Contexts

- **Status:** Draft for architectural review
- **Decision:** Adopt Domain-Driven Design (DDD) as the modeling foundation
- **Initial deployment:** Go modular monolith
- **Evolution path:** Extract bounded contexts into microservices only when justified
- **Database:** PostgreSQL with explicit data ownership
- **Last reviewed:** 2026-11-27

---

## 1. Purpose and architecture decision

This document establishes OMICX's strategic DDD model. It identifies business subdomains, proposes bounded contexts, maps current database objects to domain ownership, and records integration rules for the Go codebase.

This is a design baseline, not proof that every listed context or capability has been implemented. Existing database objects are distinguished from future capabilities.

OMICX adopts a **DDD-first modular architecture**:

1. Bounded contexts are logical business and model boundaries.
2. Each context owns its domain rules, application use cases, and data mutations.
3. Contexts communicate through explicit application contracts and, where appropriate, domain or integration events.
4. The initial deployment remains one Go application and one PostgreSQL database.
5. A context may be extracted into a microservice later; extraction is neither automatic nor mandatory for every context.
6. No context may write directly to another context's owned tables.
7. Shared-database foreign keys may be retained where they enforce important integrity rules. Cross-context database coupling must be documented as an extraction concern.

DDD identifies domain boundaries. Microservices are one possible deployment boundary around those domains. They are not the same decision.

## 2. Business domain and operating model

OMICX is a B2B SaaS platform for BPO and outsourced customer-support operators.

A BPO organization subscribes to OMICX and operates customer-support work through the platform. The BPO manages its employees and operational teams, serves multiple client companies, connects those clients' communication channels, and handles the end customers who contact those client companies.

### 2.1 Business actors

| Actor | Meaning | Platform relationship |
|---|---|---|
| OMICX platform operator | Operates the SaaS platform itself | Platform-level administration; not a BPO tenant member by default |
| BPO owner | Owns a BPO organization | Tenant owner |
| BPO administrator / manager | Administers BPO operations | Organization member with appropriate permissions |
| Team lead | Supervises an operational team | Organization member with team membership |
| BPO agent | Handles customer-support work | Organization member with one or more team memberships |
| Client company | Business served by a BPO | Belongs to a BPO; is not an OMICX tenant |
| End customer | Person contacting a client company | External customer/contact; not a BPO employee |

### 2.2 Core business relationships

- One OMICX organization represents one BPO tenant.
- One BPO organization may operate many client companies.
- A BPO member has one global user identity and at most one current BPO organization membership.
- A BPO member may belong to multiple operational teams within that BPO.
- A team may serve multiple client companies.
- A client company may be served by multiple teams.
- An end customer contacts a client company through one or more communication channels.
- BPO employees handle those interactions on behalf of the relevant client company.

The following relationships are different business concepts and must not be collapsed into one generic assignment:
- **Organization membership:** a person's affiliation with a BPO.
- **Team membership:** a BPO member's assignment to an operational team.
- **Team-client coverage:** which team serves which client company.
- **Conversation assignment:** which team or agent handles a particular conversation.

### 2.3 Membership status discrepancy

Migration 002 permits organization membership statuses of active, suspended, and ended. It does not include invited. Invitations and invitation tokens are explicitly out of scope in migration 002. A future invitation should have its own lifecycle, rather than being represented as an organization membership status without a deliberate schema change.

## 3. Ubiquitous language

| Term | Definition |
|---|---|
| OMICX Platform | The SaaS product and its platform-level operation |
| BPO Organization / Tenant | The subscribing BPO operator and tenant boundary for its operational data |
| User | A global human identity in OMICX |
| Organization Membership | A time-bounded affiliation between a user and one BPO |
| Current Membership | A membership whose status is active or suspended; suspended still occupies the user's single current-membership slot |
| Ended Membership | Historical affiliation; rejoining creates a new membership record |
| Client Company | A business whose customer-support operation is managed by a BPO |
| End Customer / Contact | A person who contacts a client company; not a global OMICX user by default |
| Team | An operational group of BPO members |
| Team Membership | A member's assignment to a team, with a team-scoped role |
| Team-Client Coverage | A relationship stating which team is responsible for serving a client company |
| Channel Connection | A configured integration with an external communication provider |
| Inbox | A configured support entry point through which conversations are received and handled; final ownership is open |
| Conversation | A support interaction involving an end customer and a client company's support operation |
| Message | An inbound, outbound, or internal communication record associated with a conversation |
| Conversation Assignment | A routing/work-distribution decision assigning work to a team or agent |
| Workflow | A versioned set of triggers, conditions, and actions executed by automation |
| AI Run | A traceable AI execution, including model and tool activity |
| Knowledge Source | An external or uploaded source from which support knowledge is derived |

## 4. Subdomain classification

This classification is a strategic hypothesis. It must be revisited using customer discovery, paid validation, and operational evidence.

### 4.1 Core subdomains

These are the proposed areas where OMICX should create differentiated value for BPO operators.

| Core subdomain | Potential differentiation |
|---|---|
| Customer Support Operations | Unified lifecycle for customer interactions across clients and channels |
| Work Distribution & Routing | Efficient allocation of customer work to eligible teams and agents |
| Automation & Operational Orchestration | Execution of repeatable support processes and reduction of manual coordination |
| AI-Assisted Support | Improved agent productivity and customer resolution while preserving human control |

Do not assume AI alone is the core differentiator. BPO workflow orchestration, routing quality, client-specific controls, measurable service delivery, and AI assistance may together create the advantage. This needs validation with paying BPO customers.

### 4.2 Supporting subdomains

| Supporting subdomain | Purpose |
|---|---|
| Organization & Tenant Management | Represent BPO tenants and membership lifecycle |
| Client Management | Manage businesses served by a BPO |
| Workforce Management | Manage teams, memberships, and operational eligibility |
| Service Coverage | Configure which teams serve which client companies |
| Channel Integration | Connect external communication providers |
| Contact Management | Resolve and maintain end-customer identities |
| Knowledge Management | Manage support information used by agents and AI |
| Quality Management | Measure service quality, SLA performance, and customer satisfaction |
| Analytics & Reporting | Provide BPO, client, team, and agent operational insight |

### 4.3 Generic subdomains

| Generic subdomain | Purpose |
|---|---|
| Identity & Authentication | Global user identity, credentials, and sessions |
| Access Control | Role/permission catalogue and authorization decisions |
| Billing & Entitlements | Subscription, plan, and product entitlement mechanics |
| Notifications | Delivery of operational notifications |
| Media Storage | Storage and retrieval of uploaded media |
| Platform Observability | Logs, metrics, traces, health, and operational diagnostics |

Generic does not mean unimportant. Authentication, authorization, billing correctness, and tenant isolation remain security- and business-critical.

## 5. Bounded context inventory

The contexts below are logical boundaries. They are not a deployment plan and do not imply one process or database per context.

### 5.1 Identity & Authentication

**Purpose:** Establish and manage global human identities and authentication sessions.

- Owns global user identity, canonical email, account status, password credentials, and refresh-token session lifecycle.
- Does not own BPO membership, team membership, or client-company relationships.
- Does not infer tenant authorization from user identity or a client-supplied organization identifier.

**Existing tables:** omicx.users, omicx.password_credentials, omicx.user_sessions.

**Representative use cases:** Register user, authenticate user, change password, rotate refresh token, revoke session, disable user.

### 5.2 Organization Management

**Purpose:** Manage BPO tenants and a user's affiliation with a BPO.

- Owns BPO organization lifecycle and organization membership lifecycle.
- Enforces the one-current-BPO-membership-per-user rule.
- Preserves ended memberships as history; rejoining creates a new membership.
- Coordinates membership termination with team-membership cleanup and session revocation through an application transaction initially, and a durable process if contexts are later distributed.
- Owns organization creation and initial owner assignment use cases.

**Existing tables:** omicx.organizations, omicx.organization_memberships.

**Representative use cases:** Create BPO, accept invitation (future separate invitation model), suspend membership, end membership, transfer ownership.

**Invariant:** Every organization must have an owner as a business invariant. Migration 004 enforces at most one current owner; it does not enforce existence of an owner. Application use cases must guarantee both existence and correct transfer behavior.

### 5.3 Access Control

**Purpose:** Define and evaluate authorization capabilities.

- Owns the role catalogue, permission catalogue, role-permission mapping, and authorization policy semantics.
- Evaluates permissions against authenticated identity, verified organization membership, team membership, and resource scope.
- Does not authenticate credentials.
- Does not own organization or team membership lifecycle.
- Role assignment references are held by the context that owns the membership record.

**Existing tables:** omicx.roles, omicx.permissions, omicx.role_permissions.

**Representative use cases:** Resolve effective permissions, authorize action, evaluate team-scoped access, enforce protected-role rules.

**Current limitation:** Migration 003 seeds predefined global roles and permissions. It explicitly defers tenant-defined custom roles. is_system = false does not itself implement custom-role ownership or isolation.

### 5.4 Client Management

**Purpose:** Manage the businesses for which a BPO provides support services.

- Owns client company identity, configuration, status, locale, timezone, and business-level settings.
- Does not own the BPO tenant or BPO employees.
- Does not own end-customer identities or conversations.

**Existing tables:** omicx.client_companies.

**Representative use cases:** Create client company, update client configuration, suspend client, archive client.

### 5.5 Workforce Management

**Purpose:** Manage operational teams and the participation of BPO members in those teams.

- Owns teams and team memberships.
- Supports a member belonging to multiple teams within the same BPO.
- Owns team-scoped role assignment and team-member lifecycle.
- Must reject or prevent active team membership for a member whose organization membership is no longer eligible.
- Future capabilities may include agent availability, skills, capacity, shifts, and scheduling; these are not present in the current schema.

**Existing tables:** omicx.teams, omicx.team_memberships.

**Representative use cases:** Create team, add member, change team role, end team membership, evaluate team eligibility.

### 5.6 Service Coverage

**Purpose:** Define which operational teams serve which client companies.

- Owns the team-client coverage relationship and its history.
- Enforces that a team and client company belong to the same BPO.
- Is distinct from assigning an individual conversation to a team or agent.

**Existing table:** omicx.team_client_assignments.

**Boundary status:** Provisional. During tactical design, validate whether this is a distinct Service Coverage context or a capability within Client Management / Service Delivery. Do not move the table or create packages until the business workflow is clear.

### 5.7 Channel Integration

**Purpose:** Integrate external communication providers and normalize provider-specific events.

- Owns provider adapters, channel connections, webhook verification, provider credentials, retry handling, provider rate limits, and delivery callbacks.
- Converts provider payloads into internal integration contracts.
- Does not own conversation business lifecycle or agent routing.
- Must treat inbound webhook delivery as potentially duplicated and out of order.

**Existing tables:** None in migrations 001–004.

**Future concepts:** ChannelConnection, ProviderAccount, WebhookEvent, DeliveryAttempt.

### 5.8 Contact Management

**Purpose:** Maintain end-customer identities and client-specific customer profiles.

- Owns customer/contact profiles and external channel identity mappings.
- Handles identity matching, duplicate detection, and controlled contact merges.
- Must not confuse a contact with a global OMICX user.
- Contact uniqueness and visibility boundaries need careful design: the same human may contact multiple client companies, while client confidentiality may require separate profiles or explicit identity linkage.

**Existing tables:** None in migrations 001–004.

**Future concepts:** Contact, ContactIdentity, ContactAttribute, ContactMerge.

### 5.9 Conversation Management

**Purpose:** Own the lifecycle and history of customer-support interactions.

- Owns conversation state, participants, messages, and conversation-level business transitions.
- Persists messages and their relationship to conversations.
- Does not implement provider-specific webhook parsing.
- Does not decide which agent is eligible to receive work.
- Does not delegate its source-of-truth responsibility to an AI model.

**Existing tables:** None in migrations 001–004.

**Future concepts:** Conversation, Message, Participant, Inbox (ownership to be confirmed), ConversationStatus.

### 5.10 Routing & Assignment

**Purpose:** Decide where customer-support work should go and track assignment decisions.

- Owns queues, routing policies, work distribution, and conversation assignment lifecycle.
- Requests workforce eligibility and service-coverage information through explicit contracts.
- Distinguishes team-client coverage from individual conversation assignment.
- Handles no eligible agent, reassignment, agent departure, and capacity constraints.

**Existing tables:** None in migrations 001–004.

**Future concepts:** Queue, RoutingPolicy, Assignment, RoutingDecision.

### 5.11 Automation Management

**Purpose:** Execute configured operational workflows.

- Owns workflow definitions, versions, triggers, conditions, actions, and execution state.
- Reacts to defined business events.
- Does not bypass the owning context when changing conversation, membership, billing, or other domain state.
- Actions that mutate another context use that context's command/application contract.

**Existing tables:** None in migrations 001–004.

### 5.12 Knowledge Management

**Purpose:** Manage information sources available to agents and AI retrieval.

- Owns sources, ingestion lifecycle, document versions, chunk metadata, and indexing state.
- Owns retrieval access policies for knowledge resources.
- Does not own conversation messages or AI model execution.

**Existing tables:** None in migrations 001–004.

### 5.13 AI Orchestration

**Purpose:** Coordinate model calls, retrieval, tools, and human handoff for support workflows.

- Owns AI runs, model/provider selection, prompt versions, tool invocation records, policy checks, and AI execution telemetry.
- May propose a reply or action but cannot become the source of truth for a conversation or message.
- Applies tenant/client context and authorization constraints to retrieval and tool execution.
- Supports timeouts, retries, cost limits, model failures, and human handoff.

**Existing tables:** None in migrations 001–004.

### 5.14 Quality Management

**Purpose:** Measure support quality and service commitments.

- Owns SLA policies and timers, scorecards, quality evaluations, and CSAT lifecycle.
- Consumes conversation and assignment facts through explicit contracts or events.
- Does not mutate conversation state directly.

**Existing tables:** None in migrations 001–004.

### 5.15 Analytics & Reporting

**Purpose:** Provide operational views for BPO owners, managers, clients, teams, and agents.

- Owns reporting read models, aggregations, metric definitions, and reporting snapshots.
- Consumes facts from operational contexts.
- Does not become a backdoor for cross-context transactional writes.
- May later need a separate analytical store when operational query load or retention requirements justify it.

**Existing tables:** None in migrations 001–004.

### 5.16 Billing & Entitlements

**Purpose:** Manage commercial subscriptions and enforce product entitlements.

- Owns plans, subscription lifecycle, entitlements, payment references, and billable usage records.
- Integrates with payment providers through adapters.
- Does not infer financial charges from incomplete analytics aggregates.
- Usage events require idempotency and reconciliation.

**Existing tables:** None in migrations 001–004.

### 5.17 Notification Management

**Purpose:** Deliver notifications to BPO members and platform operators.

- Owns notification preferences, templates, delivery attempts, and notification status.
- Consumes events from business contexts.
- Does not own the business transaction that caused the notification.

**Existing tables:** None in migrations 001–004.

## 6. Context map and integration rules

### 6.1 Main relationships

| Upstream context | Downstream context | Information / contract |
|---|---|---|
| Identity & Authentication | Organization Management | Authenticated global user identity |
| Organization Management | Access Control | Verified tenant membership context |
| Organization Management | Workforce Management | Membership lifecycle facts and identity references |
| Organization Management | Client Management | Tenant ownership and lifecycle |
| Client Management | Service Coverage | Client identity and status |
| Workforce Management | Service Coverage | Team identity and status |
| Service Coverage | Routing & Assignment | Which teams may serve a client |
| Workforce Management | Routing & Assignment | Team/agent eligibility and availability |
| Channel Integration | Conversation Management | Normalized inbound message/event |
| Contact Management | Conversation Management | Resolved contact identity |
| Conversation Management | Routing & Assignment | Work requiring routing |
| Conversation Management | Automation Management | Conversation lifecycle facts |
| Conversation Management | AI Orchestration | Authorized request for AI assistance |
| Knowledge Management | AI Orchestration | Scoped knowledge retrieval |
| AI Orchestration | Conversation Management | Proposed/approved response or tool result |
| Routing & Assignment | Notification Management | Assignment and reassignment facts |
| Conversation Management | Quality Management | Conversation and message facts |
| Quality Management | Analytics & Reporting | Quality/SLA metrics |
| Operational contexts | Billing & Entitlements | Idempotent billable usage facts |

These relationships are conceptual. Exact request/response and event contracts belong in a later integration-design document.

### 6.2 Boundary rules

1. A context is the only writer to its owned tables.
2. Other contexts use exported application interfaces or integration contracts.
3. Do not import another context's internal domain entities.
4. Cross-context references normally use stable identifiers and explicit contracts, not shared object graphs.
5. Every tenant-owned operation validates organization and client scope.
6. A context must not trust an organization identifier merely because a frontend or another context supplied it.
7. Integration events describe facts that have committed; commands request work and may be rejected.
8. Event consumers must be idempotent.
9. Cross-context workflows that eventually span services must tolerate retries and partial failure.
10. No distributed transaction is assumed.

## 7. Existing database ownership map

This mapping reflects migrations 001–004 on the current repository baseline.

| Table | Proposed owning context | Notes |
|---|---|---|
| omicx.users | Identity & Authentication | Global identity |
| omicx.password_credentials | Identity & Authentication | Password hashes |
| omicx.user_sessions | Identity & Authentication | Refresh-token hashes and session revocation |
| omicx.organizations | Organization Management | BPO tenant |
| omicx.organization_memberships | Organization Management | Current and historical BPO affiliation; stores organization role reference |
| omicx.roles | Access Control | Global predefined role catalogue |
| omicx.permissions | Access Control | Global permission catalogue |
| omicx.role_permissions | Access Control | Role-permission mappings |
| omicx.client_companies | Client Management | BPO-owned client businesses |
| omicx.teams | Workforce Management | BPO operational teams |
| omicx.team_memberships | Workforce Management | Team participation; stores team role reference |
| omicx.team_client_assignments | Service Coverage (provisional) | Historical team-to-client coverage |

### 7.1 Current schema coupling

The existing schema uses composite foreign keys to enforce same-tenant relationships. This is valuable for integrity in the current shared PostgreSQL database.

There are also cross-context references:

- organization_memberships.organization_role_id references the Access Control role catalogue.
- team_memberships.team_role_id references the Access Control role catalogue.
- team_memberships references Organization Management's organization membership.
- team_client_assignments references Workforce teams and Client Management client companies.

These constraints are acceptable as an intentional shared-database transition design. If contexts are separated into independent databases, cross-database foreign keys will no longer be possible. Future service contracts and local references must replace those guarantees. Do not remove current constraints merely to imitate microservice architecture.

### 7.2 Known schema/application invariants

- Migration 002 permits at most one current BPO membership globally per user (active or suspended).
- Migration 004 permits at most one current org_owner membership per organization (active or suspended).
- Migration 004 does not guarantee that an organization always has an owner.
- Ending an organization membership must also end active team memberships and revoke sessions through a coordinated application use case; the current schema does not fully implement this lifecycle automatically.
- Organization membership status does not currently include invited.
- Migration 003 seeds predefined roles; custom tenant-defined roles are deferred.
- PostgreSQL RLS is not implemented by migrations 001–004.
- updated_at defaults do not automatically update a row on every update; application update logic or triggers are still required.

## 8. Initial Go modular-monolith organization

The code should reflect bounded contexts without prematurely introducing distributed service infrastructure.

~~~text
backend/
├── cmd/
│   └── api/
├── internal/
│   ├── platform/
│   │   ├── config/
│   │   ├── database/
│   │   ├── httpserver/
│   │   ├── logger/
│   │   └── observability/
│   ├── identity/
│   ├── organization/
│   ├── accesscontrol/
│   ├── client/
│   ├── workforce/
│   ├── servicecoverage/
│   ├── channel/
│   ├── contact/
│   ├── conversation/
│   ├── routing/
│   ├── automation/
│   ├── knowledge/
│   ├── ai/
│   ├── quality/
│   ├── analytics/
│   ├── billing/
│   └── notification/
├── migrations/
└── tests/
~~~

This is a target package map, not a request to create empty folders now. Context packages should be created when their contracts and first use cases are ready.

Within a context, use a dependency direction such as:

- **domain:** entities, value objects, aggregates, domain services, invariants.
- **application:** use cases, commands, queries, transaction orchestration.
- **ports:** interfaces required by application/domain behavior.
- **infrastructure:** PostgreSQL repositories and external adapters.
- **transport:** HTTP or messaging adapters.

The domain layer must not import HTTP frameworks, database drivers, Redis clients, or provider SDKs.

The current Tern migrations may remain in one ordered migration directory. Each migration must document the context that owns the affected tables. A separate migration directory per context is not required for the modular monolith.

## 9. Microservice evolution strategy

DDD creates boundaries that can support service extraction; it does not guarantee that extraction is easy. A service split introduces network failures, eventual consistency, independent deployment, security, tracing, retries, and distributed workflow design.

### 9.1 Extraction candidates

| Candidate | Why it may be extracted | Evidence required |
|---|---|---|
| Channel Integration | Provider-specific traffic, webhook retries, external API failure isolation | Independent scaling/failure profile and stable normalized contracts |
| AI Orchestration | Model workloads, latency/cost isolation, provider-specific scaling | Measured resource contention or separate deployment needs |
| Analytics & Reporting | Read-heavy workloads and long-running aggregations | Operational database query interference or warehouse requirements |
| Notification Management | Independent delivery channels and retry workloads | Independent scaling/reliability requirements |
| Billing & Entitlements | Payment-provider integration and financial lifecycle | Mature usage contracts, reconciliation, and isolation requirements |

Identity, Organization, Workforce, and Access Control should not be split early. They have tightly related security and membership invariants and currently share important relational constraints.

### 9.2 Extraction readiness criteria

A context is not ready to become a microservice merely because it has a package directory. Before extraction, it should have:

- Stable domain ownership and vocabulary.
- Explicit inbound and outbound contracts.
- No external writes to its tables.
- Idempotent event handling where asynchronous.
- Defined failure and retry behavior.
- Traceable correlation and tenant context.
- Independent tests and deployment configuration.
- A database ownership and data-migration plan.
- A measurable operational or organizational reason to extract.

### 9.3 No premature infrastructure

Do not add Kafka, Kubernetes, a service mesh, distributed tracing vendors, or one database server per context solely because the architecture is microservice-ready. Add infrastructure when there is a concrete workload, reliability, security, or deployment requirement.

## 10. Architecture risks and unresolved decisions

These are explicit open decisions. They must be resolved during tactical design and domain discovery.

| ID | Open decision | Why it matters |
|---|---|---|
| DDD-OD-001 | Is Service Coverage its own context or part of Client/Service Delivery Management? | Determines ownership of team_client_assignments |
| DDD-OD-002 | Does an Inbox belong to Channel Integration or Conversation Management? | Separates provider connection from support workflow configuration |
| DDD-OD-003 | Can one end customer have a global identity across multiple client companies, or are contacts client-scoped by default? | Critical to privacy, deduplication, and data isolation |
| DDD-OD-004 | What is the exact lifecycle and ownership of a conversation assignment? | Prevents overlap between Conversation and Routing contexts |
| DDD-OD-005 | Which workload/customer requirements justify separating AI, Channel, or Analytics into services? | Prevents premature extraction |
| DDD-OD-006 | What are the invitation, membership termination, and ownership-transfer workflows? | Must preserve membership and owner invariants |
| DDD-OD-007 | What client-specific service configuration is required beyond the current client company record? | Avoids prematurely adding ClientAccount/Program hierarchy |

Do not create speculative ClientAccount, Program, Sector, or other hierarchy until a real BPO workflow requires it.

## 11. Recommended implementation sequence

1. Complete strategic design review and terminology.
2. Create a context map with upstream/downstream contracts.
3. Perform tactical design for Identity, Organization, and Access Control.
4. Perform tactical design for Client Management, Workforce, and Service Coverage.
5. Define architecture tests preventing unauthorized cross-context imports and writes.
6. Implement BPO bootstrap in one database transaction.
7. Implement authentication and authorization use cases.
8. Implement client and workforce operations.
9. Start the first customer-support vertical slice only after tenant and access boundaries are enforced.

## 12. Acceptance criteria for DDD-001

- [ ] BPO, client company, end customer, user, organization membership, team membership, and conversation assignment are unambiguously distinguished.
- [ ] Core, supporting, and generic subdomains are documented as hypotheses.
- [ ] Every existing table has a proposed owning context.
- [ ] Every bounded context has a purpose and responsibility boundary.
- [ ] Context dependencies are documented.
- [ ] Cross-context database coupling is explicitly recorded.
- [ ] Current schema limitations and deferred capabilities are documented.
- [ ] Microservice extraction criteria are explicit.
- [ ] Open decisions are recorded rather than hidden.
- [ ] No implementation folder or service is created solely to match the diagram.

**DDD-001 is a draft until its open decisions are reviewed.** Acceptance approves the strategic boundaries; it does not mean all contexts are implemented.
