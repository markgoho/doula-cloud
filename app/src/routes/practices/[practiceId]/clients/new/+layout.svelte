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
	 * and this layout opens it, seeded from what the search that fronts
	 * intake carried in its query string (#498).
	 *
	 * ## What is deliberately not here
	 *
	 * #497's manual focus effect. It existed because a step change inside
	 * one route moves no focus; a per-step route sequence gets
	 * SvelteKit's own focus reset on navigation, and porting the
	 * workaround would fight it.
	 */
	import type { Snippet } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	let { children }: { children: Snippet } = $props();

	const practiceId = $derived(page.params.practiceId ?? '');

	/*
	 * The four keys the search hands over (#498). Each lands on the page
	 * that asks for it, so a carried value is still shown and still
	 * editable before the save -- `name` is the given name, since that is
	 * the one field search matches against all three name columns.
	 */
	function carried(key: string): string {
		return page.url.searchParams.get(key)?.trim() ?? '';
	}

	$effect(() => {
		intakeDraft.start(practiceId, {
			givenName: carried('name'),
			phone: carried('phone'),
			email: carried('email'),
			dateOfBirth: carried('dateOfBirth')
		});
		void intakeFlow.load(apiFetchWithSession, practiceId);
	});
</script>

{#if intakeFlow.status === 'ready'}
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
