/**
 * The Client register (ADR-0005, CONTEXT.md's `_Client says_:` lines) as
 * code, not prose a portal screen re-derives on its own. Binding on
 * `routes/portal/**` -- see `clientRegister.usage.spec.ts` for the gate
 * that holds it there. A team word or a raw enum value earns a lookup
 * failure here rather than reaching a Client silently; there is no
 * fallback-to-raw path, unlike a Staff-facing label such as
 * `invoiceStatusLabel` -- ADR-0005's whole premise is that a Client never
 * meets the domain word, so quietly printing one on a status this build
 * has not labeled yet would be the exact defect this module exists to
 * prevent.
 *
 * Decided on #212 (comment threads from #433 and #834): the label is
 * produced here, in the Svelte layer, from this one module; the `portal`
 * API DTO keeps sending the raw enum.
 */

import { formatInstant } from './dates.js';

/** `engagement_status` has three values today (ADR-0015 superseded
 * ADR-0005's four-value set when `postpartum` left the column). One
 * fixed label per value, the same for every Client. */
const ENGAGEMENT_STATUS_LABELS: Record<string, string> = {
	intake: 'Getting started',
	active: 'Ongoing',
	completed: 'Care ended'
};

export function engagementStatusLabel(status: string): string {
	const label = ENGAGEMENT_STATUS_LABELS[status];
	if (!label) throw new Error(`clientRegister: no Client label for engagement status "${status}"`);
	return label;
}

/** `contract_status` (`draft | sent | signed | voided`) had no Client
 * register entry before #212 -- CONTEXT.md's Contract entry gains one
 * here. `draft` never reaches a Client (a Draft Contract 404s the
 * client-portal read the same way an unsent one does), but it still gets
 * a fixed label rather than being left to throw, in case that ever
 * changes. */
const CONTRACT_STATUS_LABELS: Record<string, string> = {
	draft: 'Being prepared',
	sent: 'Ready for your signature',
	signed: 'Signed',
	voided: 'No longer active'
};

export function contractStatusLabel(status: string): string {
	const label = CONTRACT_STATUS_LABELS[status];
	if (!label) throw new Error(`clientRegister: no Client label for contract status "${status}"`);
	return label;
}

/** The terminal notice a voided Contract carries for a Client (NH-G5) --
 * fixed wording naming the Practice, never the Staff `ContractStatus`
 * component's bare "Voided". Deliberately does not claim anything about
 * an Invoice: voiding a Contract (`api/internal/contracts/void.go`)
 * never touches one, so a sentence promising nothing more is owed would
 * be a fact the model does not hold -- exactly what ADR-0005 calls a lie
 * the first test run would have to assert. */
export function contractVoidedNotice(practiceName: string): string {
	return `${practiceName} ended this Contract.`;
}

/** The Engagement noun (CONTEXT.md: "my care", heading form "Your care").
 * `CARE_HEADING` replaces every "Choose an Engagement" heading; `NO_CARE_MESSAGE`
 * replaces every "You don't have an Engagement yet" paragraph beside it. */
export const CARE_HEADING = 'Your care';
export const NO_CARE_MESSAGE = "You don't have care set up yet. Ask your Practice to set it up.";

/**
 * The one label a Client reads for an Engagement wherever more than one of
 * hers might sit side by side: the portal root's list, and the authenticated
 * chrome's own way back to it (#310). Naming by Practice alone stopped being
 * enough once ADR-0015 let two Engagements share a Practice, so the label
 * also carries when this one began -- CONTEXT.md's register addition for
 * #310, and the one fact that distinguishes them honestly for every Client,
 * including one whose care ended in loss (unlike a due date or a birth
 * outcome, both staff-only or absent). Deliberately narrow: the input type
 * only has room for what the register allows, so a caller cannot pass a
 * staff-only fact in even by accident.
 */
export interface EngagementLabelInput {
	practiceName: string;
	createdAt: string;
}

export function engagementLabel(engagement: EngagementLabelInput): string {
	return `${engagement.practiceName}, started ${formatInstant(engagement.createdAt)}`;
}

/**
 * The Client register's own phrase for one `activity.EngagementAction`
 * (#708). One fixed phrase per action, the same for every Client, in the
 * vocabulary CONTEXT.md's `_Client says_:` lines already settle -- her
 * care, her visits, her Contract, her Invoice, her payments, her Birth
 * Plan, her login, her notifications.
 *
 * This is deliberately the hand-maintained per-action table
 * `describeActivityAction` argues against for the two staff surfaces, and
 * that argument does not carry here: the humanizer's whole output is the
 * raw action string with its underscores taken out, which is a domain
 * word, and ADR-0005's premise is that a Client never meets one. What
 * keeps this table from going stale is not a generic fallback but
 * `activityPhrases.usage.spec.ts`, which reads the Go action vocabulary
 * itself and fails on an action added, removed, or left unphrased.
 *
 * Actor-neutral throughout: the ledger's own "Who" column already carries
 * the actor (a Client's own name, "Your practice", or "Doula Cloud"), so
 * a phrase naming one would say it twice, and several of these actions
 * can be either kind.
 *
 * Keyed by every action a Client can reach -- every `EngagementAction`
 * except the staffing set `activity.StaffingActions()` names, which the
 * portal reader excludes in SQL before a row ever gets here.
 */
const CLIENT_ACTIVITY_PHRASES: Record<string, string> = {
	// The Engagement, in the register's own word for it: "my care".
	// `care_phase_changed` is the intake -> active move and only that, so
	// its phrase can name where the care landed rather than hedge; the
	// word is `active`'s own fixed label above, "Ongoing".
	engagement_created: 'Your care was set up.',
	care_phase_changed: 'Your care is now ongoing.',
	engagement_completed: 'Your care ended.',

	// The Contract entity's own lifecycle. `contract_voided` reads as
	// `voided`'s own fixed label ("No longer active") rather than
	// borrowing `contractVoidedNotice`'s wording, which names the
	// Practice -- a name this row does not carry.
	contract_created: 'Your Contract was prepared.',
	contract_sent: 'Your Contract was sent to you.',
	contract_signed: 'Your Contract was signed.',
	contract_voided: 'Your Contract is no longer active.',

	// Its price. `contract_amount_overridden` (a person chose a different
	// figure) and `contract_amount_repriced` (the Practice's rate card
	// moved underneath it) share one phrase on purpose: the difference
	// between them is the Practice's own bookkeeping, and to a Client both
	// mean her price changed -- the same reasoning CONTEXT.md's Invoice
	// entry gives for `void` and `uncollectible` sharing "No longer owed".
	contract_priced: "Your Contract's price was set.",
	contract_amount_overridden: "Your Contract's price changed.",
	contract_amount_repriced: "Your Contract's price changed.",

	// #971's two workflow acts. Both are the Practice talking to itself,
	// and both reach her ledger today because neither is in the staffing
	// set -- phrased here rather than left to throw, with whether they
	// should reach her at all left to #1096, which is a filter question
	// rather than a wording one.
	contract_void_requested: 'Your Practice asked to end your Contract.',
	contract_void_declined: 'Your Practice decided to keep your Contract.',

	// Visits, in her own phrasing. `visit_notes_edited` says notes exist
	// and never what they say: ADR-0006 keeps a Visit's notes staff-only,
	// and a phrase promising her a reading of them would be a fact the
	// model does not hold.
	visit_logged: 'A visit was added to your care.',
	visit_scheduled: "A visit's date and time changed.",
	visit_notes_edited: 'Notes were added to a visit.',

	// The Plan Instance, which she calls her Birth Plan (CONTEXT.md's Plan
	// Instance entry: she never meets the concept itself).
	plan_instance_edited: 'Your Birth Plan was updated.',
	birth_plan_acknowledged: 'You marked your Birth Plan as read.',

	// Her money. `invoice_voided` and `invoice_written_off` share one
	// phrase for CONTEXT.md's own stated reason: "Written off" would tell
	// her that her Practice absorbed a loss on her, which is true, unkind,
	// and hers to be spared.
	invoice_raised: 'You were sent an Invoice.',
	invoice_paid: 'An Invoice was paid.',
	payment_recorded: 'A payment from you was recorded.',
	payment_reversed: 'A payment recorded earlier was removed.',
	invoice_voided: 'An Invoice is no longer owed.',
	invoice_written_off: 'An Invoice is no longer owed.',

	// Her way in. Never "portal": CONTEXT.md's Portal Account entry says
	// she meets a login and never the term, and no Client-facing copy in
	// this build says "portal" either.
	portal_invite_sent: 'You were invited to sign in.',
	portal_account_provisioned: 'Your login was created.',
	portal_account_linked: 'This care was added to your login.',

	// #303, in the notifications screen's own two words for the control.
	push_notifications_enabled: 'You turned notifications on.',
	push_notifications_disabled: 'You turned notifications off.'
};

export function clientActivityPhrase(action: string): string {
	const phrase = CLIENT_ACTIVITY_PHRASES[action];
	if (!phrase) throw new Error(`clientRegister: no Client phrase for activity action "${action}"`);
	return phrase;
}

/** The table's own keys, sorted, for the drift guard that compares them
 * against the Go action vocabulary. Exported for that spec alone -- no
 * screen has a reason to enumerate the actions, only to phrase the one
 * row it holds. */
export function clientActivityPhrasedActions(): string[] {
	return Object.keys(CLIENT_ACTIVITY_PHRASES).toSorted((a, b) => a.localeCompare(b));
}
