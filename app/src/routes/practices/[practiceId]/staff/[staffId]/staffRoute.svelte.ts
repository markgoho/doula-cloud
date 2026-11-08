import { page } from '#lib/appState.svelte.js';
import { isOwner as checkIsOwner } from '#lib/roles.js';
import type { PracticeSession } from '../../+layout.js';

/**
 * What the Staff member's page and its question pages each read off the
 * address and the session: the Practice, the Staff member, and whether the
 * signed-in person owns the Practice. Call it while a component initializes.
 */
export function staffRoute() {
	const practiceId = $derived(page.params.practiceId!);
	const staffId = $derived(page.params.staffId!);
	const isOwner = $derived(checkIsOwner((page.data as { session: PracticeSession }).session));
	return {
		get practiceId() {
			return practiceId;
		},
		get staffId() {
			return staffId;
		},
		get isOwner() {
			return isOwner;
		}
	};
}
