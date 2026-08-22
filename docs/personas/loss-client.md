# Nadia Haddad — client whose pregnancy ends in loss

- **Archetype**: Live Engagement ends in stillbirth at 31 weeks
- **Pronouns**: she/her
- **Surface**: client portal (`/portal`), and then mostly not
- **Roles**: none — she is a **Client** with an `active` Engagement
- **Entry point**: already in the portal; the journey turns mid-way, not at the start

## Who she is

Nadia is 36, four months into an Engagement with Maya, with a signed Contract, a filled
Birth Plan, and eleven Visits logged. At 31 weeks her baby's heart stops. She still
gives birth. Maya is there.

Afterwards Nadia does not open the portal for three weeks. When she does, she does not
want to be asked how her third trimester is going.

## Why this persona exists

Because software written only for the happy path will hurt her, automatically and at
scale, at the worst moment of her life. This is the persona that finds the cruelty.

## Primary journey

An Engagement that ends without a living baby: the Practice marks what happened, the
app stops behaving as though a birth is coming, postpartum support continues, and the
record closes without erasing her.

## Done looks like

The Engagement is closed in a way that is accurate, Maya can still support her through
postpartum, and nothing in the app — no prompt, no reminder, no notification, no label —
addresses her as though the pregnancy is ongoing.

## Watch for

- **`engagement_status` has no terminal state for this.** The enum is
  `intake | active | postpartum | completed`. Marking her `completed` says the work
  finished; leaving her `active` says the pregnancy continues. Both are wrong. This is
  the sharpest expected gap in the effort.
- Deletion is not the answer. The Engagement is a permanent record and Messages are
  immutable by design (CONTEXT.md, ADR-0002). She must be able to leave without the
  record being destroyed.
- The Birth Plan still exists and is still readable in the portal. Decide, and record,
  what should happen to it.
- Every automated prompt is a hazard: Invoice reminders, Visit reminders, push
  notifications, any "your baby is due in N weeks" copy. Walk them all.
- Billing after a loss is its own decision — what is owed, and who has to say so.
- **Treat this journey map as writing for a real person in grief.** The wording of every
  status, label, and empty state on this path is part of the deliverable.
