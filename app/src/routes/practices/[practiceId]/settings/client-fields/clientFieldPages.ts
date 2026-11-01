/**
 * What the Client Fields list and its question pages share (#2193).
 */
import { resolve } from '$app/paths';

/**
 * What a save says on the list (#2194's words): a Client's values are read
 * live (GLOSSARY.md, Client Field Template), so the questions reach every
 * Client's record at once, and the archive rule is what keeps a recorded
 * fact.
 */
export const clientFieldsSavedMessage =
	"Client Fields saved. These questions are on every Client's record now, and an archived field keeps what was already recorded.";

/**
 * What archiving a question says on the list (#2572): the question taken off,
 * and the dialog's own promise about what a Client already answered.
 */
export function clientFieldsArchivedMessage(question: string): string {
	return `"${question}" archived. New Clients will not be asked it, and a Client who already answered keeps that answer, read-only on the Client's record.`;
}

export function clientFieldsHref(practiceId: string): string {
	return resolve('/practices/[practiceId]/settings/client-fields', { practiceId });
}
