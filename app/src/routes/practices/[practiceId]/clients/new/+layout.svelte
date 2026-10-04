<script lang="ts">
	/*
	 * Intake (#466, #1611, ADR-0017): the name question, and the duplicate
	 * page a save with a match opens.
	 *
	 * ## Why the draft lives outside SvelteKit's load
	 *
	 * There is no server record until the save, so there is nothing for a
	 * `load` to return. `intakeDraft.svelte.ts` holds what has been typed
	 * and this layout opens it, seeded from what the search carried in
	 * its query string (#498). Its own `+layout.ts` is only the contractor
	 * gate (#1609).
	 *
	 * Intake reads nothing from the server before it asks (#1611): the
	 * name question is the same at every Practice, and the Practice's own
	 * sections are asked from her record (#1610). So there is no skeleton
	 * here.
	 */
	import { untrack, type Snippet } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { intakeDraft, type IntakeAnswers } from '#lib/intakeDraft.svelte.js';
	import { carriedGivenName, intakeOrigin, searchedName } from '#lib/intakeJourney.js';
	import ContractorDoor from '../ContractorDoor.svelte';
	import type { LayoutProps as LayoutProperties } from './$types';

	// #1609: `+layout.ts` decided whether she is a contractor Doula.
	// `data` is optional for the same reason `clients/+page.svelte` gives:
	// a spec that renders this directly bypasses SvelteKit's load.
	let { data, children }: { data?: LayoutProperties['data']; children: Snippet } = $props();
	const isContractor = $derived(data?.isContractor ?? false);

	const practiceId = $derived(page.params.practiceId ?? '');

	/*
	 * The four keys the search hands over (#498). `name` is the given
	 * name when it is one word, and the name question shows it in that
	 * field. A value of two or more words carries into neither name
	 * field: it is never split on a space (#1716, `carriedGivenName`),
	 * and the name question shows it, read-only (#1758, `searchedName`).
	 * The other three are listed on the name question and saved with it
	 * (#1611).
	 */
	function carried(): Partial<IntakeAnswers> {
		const seeded: Partial<IntakeAnswers> = {};
		const givenName = carriedGivenName(page.url.searchParams.get('name'));
		if (givenName) seeded.givenName = givenName;
		for (const [parameter, key] of [
			['phone', 'phone'],
			['email', 'email'],
			['dateOfBirth', 'dateOfBirth']
		] as const) {
			const value = page.url.searchParams.get(parameter)?.trim();
			// A key search did not carry is left off entirely. Seeding it as
			// an empty string would be a value, and a value overwrites.
			if (value) seeded[key] = value;
		}
		return seeded;
	}

	/*
	 * `untrack`, because `start` READS the draft to decide whether to seed
	 * it. Without this the effect depends on the given name, so it re-runs
	 * on every keystroke -- and a reader who clears the given name to
	 * retype it would have the draft re-seeded from a query string that is
	 * no longer there. The Practice is the only thing this should react to.
	 */
	$effect(() => {
		void practiceId;
		// The door asks nothing, so it opens no draft.
		if (isContractor) return;
		untrack(() => {
			intakeDraft.start(practiceId, carried());
			// #1609: a link that opens the name question directly names the
			// screen it is on, and the name question's Back goes there. A
			// later visit that carries none -- the duplicate page's Back, a
			// reload -- keeps the one already read.
			intakeDraft.origin = intakeOrigin(page.url.searchParams) ?? intakeDraft.origin;
			// #1758: a name of two or more words carries into no field,
			// and the name question shows it instead. A URL with no `name`
			// keeps the one already read, as above; a URL with one replaces
			// it, so a one-word name never sits under an old search's line.
			const name = page.url.searchParams.get('name');
			if (name !== null) intakeDraft.searchedName = searchedName(name);
		});
	});
</script>

{#if isContractor}
	<ContractorDoor />
{:else}
	{@render children()}
{/if}
