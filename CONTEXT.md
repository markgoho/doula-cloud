# Doula Cloud

Multi-tenant CRM for doula practices — Practices employ Staff who work Engagements with Clients through pregnancy and postpartum, tracked through Visits, plans, and billing.

## Language

**Practice**:
A tenant business. May be a solo doula or a multi-doula business with non-doula Staff.
_Avoid_: Business, tenant, org

**Staff**:
A person who works at a Practice. Holds one or more roles (e.g. Doula, Admin, Owner) via their membership at that Practice; the same person's roles may differ across Practices.
_Avoid_: User, employee, member

**Doula**:
A role a Staff member holds, not a separate entity. A Staff member with the Doula role is the one who works Engagements and Visits with Clients.
_Avoid_: Provider, practitioner

**Client**:
The pregnant/birthing person a Practice serves. Does not (yet) cover a partner or support person — portal access for a second person is a future extension of Client, not a new entity.
_Avoid_: Patient, customer, mom

**Engagement**:
The relationship between a Client and a Practice, spanning intake through postpartum, centered on one baby (born or unborn). Deliberately generic so it fits both birth-doula and postpartum-doula work.
_Avoid_: Pregnancy (too narrow — excludes postpartum-only work), Case, Relationship

**Visit**:
A scheduled meeting between a Doula and a Client within an Engagement. May be the birth itself.
_Avoid_: Appointment (reads clinical/medical-provider)

**Care Plan**:
Internal working notes on how the Practice will support a Client's Engagement. Structure is defined per-Practice via a Plan Template (see below); staff-only, never shown in the Client portal.
_Avoid_: Plan, notes

**Birth Plan**:
A standalone document of the Client's labor/delivery preferences, meant to be handed to a third party (e.g. hospital staff). Distinct from Care Plan. Structure is defined per-Practice via a Plan Template; staff-drafted, with a read-only view in the Client portal and a print stylesheet for handoff.
_Avoid_: Plan (ambiguous with Care Plan)

**Plan Template**:
A Practice's own field definitions (short text, long text, single-select, multi-select, checkbox, section header) for its Care Plan or Birth Plan, editable via a settings screen. One template per Practice per plan type, shared across all its Engagements. New Practices get a seeded default. See [ADR-0001](docs/adr/0001-practice-defined-plan-templates.md).
_Avoid_: Form, schema (schema is an implementation term, not domain language)

**Plan Instance**:
A Client's filled-out Care Plan or Birth Plan for an Engagement. Stores a snapshot of the Plan Template's field definitions as they were at creation, so later template edits never alter or break an already-completed plan.
_Avoid_: Response, submission

**Contract**:
The signed agreement governing an Engagement.
_Avoid_: Agreement

**Invoice**:
A bill issued against a Contract.
_Avoid_: Bill

**Payment**:
Money received against an Invoice.
_Avoid_: Transaction

**Message**:
Staff-to-client, bidirectional, in-app communication tied to an Engagement — one continuous thread per Engagement, not split by topic. May carry an image or PDF attachment. Immutable once sent (no edit, no delete) and kept indefinitely as part of the Engagement's permanent record. Delivered via push-triggered fetch: a content-free push notification wakes the client, which then fetches the real content — not a substitute for a phone call in a time-critical situation. See [ADR-0002](docs/adr/0002-message-transport-push-triggered-fetch.md).
_Avoid_: Chat, DM (implies a general-purpose messenger, not an Engagement-scoped record)
