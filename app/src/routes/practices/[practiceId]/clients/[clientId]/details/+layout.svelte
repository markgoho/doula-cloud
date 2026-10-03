<script lang="ts">
	/*
	 * A Client's details journey, as a sequence of routes (#1610,
	 * ADR-0017's amendment of 2026-10-02).
	 *
	 * Every step needs the same three facts -- her record on file, the
	 * Practice's own name and its Client Field Template -- because each
	 * page starts from the value on file and the journey's length is
	 * derived from the template. Reading them once here, and showing the
	 * skeleton once here, is what stops a reader four questions in from
	 * meeting a loading state on every navigation (ADR-0020). The same
	 * shape as `clients/new/+layout.svelte`.
	 *
	 * ## Who is refused, and where
	 *
	 * `edit.go` is what refuses: an erased Client, a merged Client, and a
	 * Client a contractor Doula is not attached to (`CanAccessClient`,
	 * whose read answers 404 to her). This layout only keeps a reader from
	 * typing answers the save would refuse: a merged record goes to the
	 * record it became, an erased one says why nothing can be added, and a
	 * record she cannot read is the read's own refusal.
	 */
	import { untrack, type Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	let { children }: { children: Snippet } = $props();

	const practiceId = $derived(page.params.practiceId ?? '');
	const clientId = $derived(page.params.clientId ?? '');
	const record = $derived(clientDetails.record);
	const loadError = $derived(clientDetails.loadError || intakeFlow.loadError);
	const isReady = $derived(clientDetails.status === 'ready' && intakeFlow.status === 'ready');

	/*
	 * `untrack`, because each `load` reads its own state to decide whether
	 * it has anything to do. The Practice and the Client are the only
	 * things this should react to.
	 */
	$effect(() => {
		const ids = { practiceId, clientId };
		untrack(() => {
			void clientDetails.load(apiFetchWithSession, ids.practiceId, ids.clientId);
			void intakeFlow.load(apiFetchWithSession, ids.practiceId);
		});
	});

	// A tombstoned row is redirected, never asked about (ADR-0017's
	// amendment): its record is the one it was merged into.
	$effect(() => {
		const survivor = record?.mergedInto;
		if (survivor) {
			void goto(resolve('/practices/[practiceId]/clients/[clientId]', { practiceId, clientId: survivor }));
		}
	});
</script>

{#if loadError}
	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<Notice variant="error" message={loadError} />
		</center-l>
	</container-l>
{:else if isReady && record?.erasedAt}
	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<stack-l space="var(--space-4)">
				<Notice
					variant="info"
					message="This Client's data was erased on request, so no details can be added."
				/>
				<Link
					href={resolve('/practices/[practiceId]/clients/[clientId]', { practiceId, clientId })}
					label="Back to the Client's record"
				/>
			</stack-l>
		</center-l>
	</container-l>
{:else if isReady && !record?.mergedInto}
	{@render children()}
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
