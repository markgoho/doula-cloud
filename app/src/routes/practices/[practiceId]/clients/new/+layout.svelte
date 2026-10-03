<script lang="ts">
	/*
	 * Intake, as a sequence of routes (#466, ADR-0017).
	 *
	 * ## Why a layout at all
	 *
	 * Every step needs the same two facts -- the Practice's own name and
	 * its Client Field Template -- because the journey's length is
	 * derived from them (#432: a Practice that has added nothing gets
	 * five steps, not six with an empty one). Reading them once here, and
	 * showing the skeleton once here, is what stops a reader four
	 * questions in from meeting a loading state on every navigation
	 * (ADR-0020).
	 *
	 * ## Why the draft lives outside SvelteKit's load
	 *
	 * There is no server record until the save, so there is nothing for a
	 * `load` to return. `intakeDraft.svelte.ts` holds what has been typed
	 * and this layout opens it, seeded from what the search carried in
	 * its query string (#498). Its own `+layout.ts` is only the contractor
	 * gate (#1609).
	 *
	 * ## What is deliberately not here
	 *
	 * #497's manual focus effect. It existed because a step change inside
	 * one route moves no focus; a per-step route sequence gets
	 * SvelteKit's own focus reset on navigation, and porting the
	 * workaround would fight it.
	 */
	import { untrack, type Snippet } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { intakeDraft, type IntakeAnswers } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import { intakeOrigin } from '#lib/intakeJourney.js';
	import ContractorDoor from '../ContractorDoor.svelte';
	import type { LayoutProps as LayoutProperties } from './$types';

	// #1609: `+layout.ts` decided whether she is a contractor Doula.
	// `data` is optional for the same reason `clients/+page.svelte` gives:
	// a spec that renders this directly bypasses SvelteKit's load.
	let { data, children }: { data?: LayoutProperties['data']; children: Snippet } = $props();
	const isContractor = $derived(data?.isContractor ?? false);

	const practiceId = $derived(page.params.practiceId ?? '');

	/*
	 * The four keys the search hands over (#498). Each lands on the page
	 * that asks for it, so a carried value is still shown and still
	 * editable before the save -- `name` is the given name, since that is
	 * the one field search matches against all three name columns.
	 */
	function carried(): Partial<IntakeAnswers> {
		const seeded: Partial<IntakeAnswers> = {};
		for (const [parameter, key] of [
			['name', 'givenName'],
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
		// The door asks nothing, so it reads nothing: the template read
		// would only race it with an error notice.
		if (isContractor) return;
		untrack(() => {
			intakeDraft.start(practiceId, carried());
			// #1609: a link that opens the name question directly names the
			// screen it is on, and the name question's Back goes there. A
			// later visit that carries none -- a Change round trip, a reload
			// further on -- keeps the one already read.
			intakeDraft.origin = intakeOrigin(page.url.searchParams) ?? intakeDraft.origin;
			void intakeFlow.load(apiFetchWithSession, practiceId);
		});
	});
</script>

{#if isContractor}
	<ContractorDoor />
{:else if intakeFlow.status === 'ready'}
	{@render children()}
{:else if intakeFlow.status === 'error'}
	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<Notice variant="error" message={intakeFlow.loadError} />
		</center-l>
	</container-l>
{:else}
	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<Skeleton lines={6} variant="text" label="Loading the questions to ask" />
		</center-l>
	</container-l>
{/if}

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}
	}
</style>
