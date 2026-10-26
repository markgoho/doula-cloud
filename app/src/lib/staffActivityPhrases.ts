/**
 * What one activity row says to a Staff reader (#2006): what happened, in
 * the words a person would say, naming what it happened to.
 *
 * Before this, the staff ledger printed the write side's own action
 * string with its underscores taken out -- "Plan instance edited",
 * "Contract priced", "Created" -- and named no Client, so a Practice-wide
 * feed of a hundred rows could not say whose contract was sent. The brief
 * calls the activity ledger this product's one signature component, so
 * every action now has its own words here, the way the Client register
 * already has its own (`clientRegister.ts`, #708).
 *
 * The subject is named by `subjectName`, which the Practice-wide feed
 * resolves server-side: the Client's full name for an Engagement's or a
 * Client record's row (the preferred name, then the family name, so two
 * Clients called Sarah read apart -- #2095, the brief's Voice amendment
 * of 2026-10-26), the staff member's name for a Membership row. The Engagement page's own ledger sends no name -- every
 * row there is about the Client the page is already headed with -- so
 * each phrase falls back to the domain noun ("the Client", "a Staff
 * member"), the brief's Voice rule 2. A Practice's own rows name the
 * setting that changed and need no name.
 *
 * No phrase names the actor: the Who column already does. No phrase uses
 * a pronoun (Voice rule 1).
 *
 * `staffActivityPhrases.usage.spec.ts` reads the Go action vocabulary and
 * fails when an action has no phrase here, or a phrase outlives its
 * action -- the same guard `activityPhrases.usage.spec.ts` holds over the
 * Client register.
 */

type Phrase = (name: string | undefined) => string;

/**
The Client a row is about, by name or by the domain noun.
*/
const client = (name: string | undefined): string => name ?? 'the Client';
/**
The same, possessive.
*/
const clients = (name: string | undefined): string => (name ? `${name}'s` : "the Client's");
/**
The staff member a Membership row is about.
*/
const member = (name: string | undefined): string => name ?? 'a Staff member';

/**
A Practice row's phrase: it names a setting, never a person.
*/
const fixed =
	(words: string): Phrase =>
	() =>
		words;

const STAFF_ACTIVITY_PHRASES: Record<string, Phrase> = {
	// The Engagement -- the care a Practice gives one Client.
	engagement_created: (name) => `Care for ${client(name)} started`,
	care_phase_changed: (name) => `Care phase changed for ${client(name)}`,
	engagement_completed: (name) => `Care for ${client(name)} ended`,
	engagement_reopened: (name) => `Care for ${client(name)} reopened`,
	kind_changed: (name) => `Type of care changed for ${client(name)}`,
	birth_outcome_recorded: (name) => `Birth outcome recorded for ${client(name)}`,
	doula_attached: (name) => `Doula added to ${clients(name)} care`,

	// The Contract.
	contract_created: (name) => `Contract drafted for ${client(name)}`,
	contract_sent: (name) => `Contract sent to ${client(name)}`,
	contract_signed: (name) => `Contract signed by ${client(name)}`,
	contract_voided: (name) => `Contract for ${client(name)} voided`,
	contract_priced: (name) => `Price set on the contract for ${client(name)}`,
	contract_amount_overridden: (name) => `Contract price for ${client(name)} changed by hand`,
	contract_amount_repriced: (name) => `Contract price for ${client(name)} changed to the new rate`,
	contract_void_requested: (name) => `Asked to void the contract for ${client(name)}`,
	contract_void_declined: (name) => `Request to void the contract for ${client(name)} declined`,

	// Visits and the Birth Plan.
	visit_logged: (name) => `Visit added for ${client(name)}`,
	visit_reassigned: (name) => `Visit for ${client(name)} given to another doula`,
	visit_scheduled: (name) => `Visit time for ${client(name)} updated`,
	visit_notes_edited: (name) => `Visit notes for ${client(name)} updated`,
	// One action per plan (#2066), so every reader names the plan.
	care_plan_edited: (name) => `Care Plan for ${client(name)} changed`,
	birth_plan_edited: (name) => `Birth Plan for ${client(name)} changed`,
	birth_plan_acknowledged: (name) => `Birth Plan marked as read by ${client(name)}`,

	// Offers of the work to a doula.
	offer_sent: (name) => `Offer sent for ${clients(name)} care`,
	offer_accepted: (name) => `Offer for ${clients(name)} care accepted`,
	offer_declined: (name) => `Offer for ${clients(name)} care declined`,
	offer_superseded: (name) => `Offer for ${clients(name)} care replaced by a newer one`,
	offer_withdrawn: (name) => `Offer for ${clients(name)} care withdrawn`,

	// Money.
	invoice_raised: (name) => `Invoice created for ${client(name)}`,
	invoice_paid: (name) => `Invoice for ${client(name)} paid`,
	payment_recorded: (name) => `Payment from ${client(name)} recorded`,
	payment_reversed: (name) => `Payment from ${client(name)} removed`,
	payment_refunded: (name) => `Money returned to ${client(name)}`,
	invoice_voided: (name) => `Invoice for ${client(name)} voided`,
	invoice_written_off: (name) => `Invoice for ${client(name)} written off`,

	// The Client's own login and notifications.
	portal_invite_sent: (name) => `${client(name)} invited to sign in`,
	portal_account_provisioned: (name) => `Login created for ${client(name)}`,
	portal_account_linked: (name) => `Care added to ${clients(name)} login`,
	push_notifications_enabled: (name) => `Notifications turned on by ${client(name)}`,
	push_notifications_disabled: (name) => `Notifications turned off by ${client(name)}`,

	// On call.
	coverage_gap_created: (name) => `Gap in on-call cover added for ${client(name)}`,
	coverage_gap_updated: (name) => `Gap in on-call cover changed for ${client(name)}`,
	coverage_gap_cleared: (name) => `Gap in on-call cover closed for ${client(name)}`,
	on_call_narrowing_changed: (name) => `Doulas on call for ${client(name)} changed`,
	on_call_rule_changed: (name) => `On-call rule for ${client(name)} changed`,

	// A Client record itself (client/events.go). An erased record's name
	// is gone by design (ADR-0027), so that phrase names none.
	created: (name) => `Client record created for ${client(name)}`,
	updated: (name) => `Client record for ${client(name)} updated`,
	merged: (name) => `Another record merged into ${clients(name)}`,
	absorbed: (name) => `${clients(name)} duplicate record merged into another`,
	erased: fixed('Client record erased'),
	portal_sign_in_address_changed: (name) => `Sign-in email changed by ${client(name)}`,
	portal_sessions_ended: (name) => `${client(name)} signed out of every device`,

	// A Membership: one person's standing at the Practice.
	joined: (name) => `${member(name)} joined the Practice`,
	roles_changed: (name) => `Roles changed for ${member(name)}`,
	employment_type_changed: (name) => `Employment type changed for ${member(name)}`,
	removed: (name) => `${member(name)} removed from the Practice`,
	sessions_ended: (name) => `${member(name)} signed out of every device`,

	// The Practice's own settings.
	stripe_connect_form_returned: fixed('Came back from the Stripe setup form'),
	stripe_connect_nudge_requested: fixed('Practice Owners asked to connect Stripe'),
	payment_terms_changed: fixed('Payment terms changed'),
	billing_mode_changed: fixed('How the Practice takes payment changed'),
	practice_rate_changed: fixed('Practice rates changed'),
	practice_name_changed: fixed('Practice name changed'),
	practice_timezone_changed: fixed('Practice time zone changed'),
	practice_on_call_changed: fixed('On-call settings changed'),
	practice_data_exported: fixed('Practice data downloaded'),
	practice_deletion_requested: fixed('Practice set to be deleted'),
	practice_deletion_restored: fixed('Practice deletion canceled'),
	practice_deletion_finalized: fixed('Practice deleted'),
	terms_accepted: fixed('Terms of Service and Privacy Policy agreed to'),
	second_factor_refused: fixed('Stopped until two-factor authentication is set up'),
	mfa_required_enabled: fixed('Two-factor authentication made a must for all Staff'),
	mfa_required_disabled: fixed('Two-factor authentication no longer a must for all Staff')
};

/**
The first letter up, for a phrase that opens on "the Client".
*/
function sentenceCase(text: string): string {
	return text.charAt(0).toUpperCase() + text.slice(1);
}

/**
 * The words for one row. Throws on an action this table does not hold,
 * the way `clientActivityPhrase` does, rather than printing the raw
 * action: the usage spec is what keeps that from reaching a screen.
 */
export function staffActivityPhrase(action: string, subjectName?: string): string {
	const phrase = STAFF_ACTIVITY_PHRASES[action];
	if (!phrase) throw new Error(`staffActivityPhrases: no Staff wording for activity action "${action}"`); // voice:ignore: a developer-facing error, never rendered
	return sentenceCase(phrase(subjectName || undefined));
}

/**
The table's own keys, sorted, for the usage spec alone.
*/
export function staffActivityPhrasedActions(): string[] {
	return Object.keys(STAFF_ACTIVITY_PHRASES).toSorted((a, b) => a.localeCompare(b));
}
