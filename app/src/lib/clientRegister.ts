/**
 * The Client register (ADR-0005, CONTEXT.md's `_Client says_:` lines) as
 * code, not prose a portal screen re-derives on its own. Binding on
 * `routes/portal/**` -- see `clientRegister.usage.spec.ts` for the gate
 * that holds it there. A team word or a raw enum value earns a lookup
 * failure here rather than reaching a Client silently; there is no
 * fallback-to-raw path, unlike a Staff-facing label such as
 * `invoiceStatusLabel` -- ADR-0005's whole premise is that a Client never
 * meets the domain word, so quietly printing one on a status this build
 * has not labelled yet would be the exact defect this module exists to
 * prevent.
 *
 * Decided on #212 (comment threads from #433 and #834): the label is
 * produced here, in the Svelte layer, from this one module; the `portal`
 * API DTO keeps sending the raw enum.
 */

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
