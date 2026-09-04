# The shell has no notification bell

[#452](https://github.com/markgoho/doula-cloud/issues/452) built the application shell and left a slot
between the Practice switcher and the avatar where a notification bell would go. It put nothing in it,
and deferred the question to [#454](https://github.com/markgoho/doula-cloud/issues/454): does
Notification grow an in-app voice, or does the bell show something else?

The answer is neither. **There is no bell.**

## What Notification actually is

Notification has no struct and no table anywhere in `api/`. The word names a pattern, not a record: a
per-domain outbox worker registered through `outbox.Register`, which sends one content-free, one-way
push or email alert and forgets it. This is what
[ADR-0009](0009-notification-is-one-term-two-voices-keyed-by-recipient.md) and
[ADR-0002](0002-message-transport-push-triggered-fetch.md) describe, and the code matches them.

Nothing durable and per-recipient exists for a bell to poll. There is no read state, no feed query, no
row that belongs to a person rather than to a send attempt.

## Why not build one

[The design brief](../design/brief.md) already decided the general case, under Goal-Gradient and
Zeigarnik: *"an unfinished Engagement appears in a list, never as a badge that pulses or a banner that
follows a person around."* A bell with an unread count is exactly the badge that rule refuses. Adding
one would not be an extension of the brief; it would be an override of it.

The three alternatives were weighed and rejected:

- **Notification grows a durable in-app voice.** This means a per-recipient table, read/unread state, a
  feed endpoint and a read gate — and it means amending ADR-0009 and ADR-0002 so that "content-free"
  becomes channel-dependent, because an in-app item that may not name a Client or an Engagement is
  nearly useless. The most expensive option, and it trades away a settled definition.
- **The bell shows unread Messages.** Messages carry no read state today, so this depends on schema that
  [#455](https://github.com/markgoho/doula-cloud/issues/455) deliberately does not build: read state is
  per-person, and the question a Practice asks — *which Clients are waiting on a reply* — is answered by
  thread authorship instead.
- **The bell shows the Activity feed.** The brief places Activity low on the page precisely because it
  is ambient. Promoting it into a badge contradicts both the Motion rule and the brief's own placement
  of that feed.

## What this means

Unfinished work is found where the work lives. A Client waiting on a reply appears in a roll-up block on
the Practice landing page ([#455](https://github.com/markgoho/doula-cloud/issues/455)); Practice-wide
Activity appears low on the hub ([#486](https://github.com/markgoho/doula-cloud/issues/486)). Both are
lists a person reads when they choose to, which is what the brief asks for.

The shell's reserved slot stays empty. A future decision may fill it, but it needs its own ADR and it
has to say what it is doing to the Motion rule.
