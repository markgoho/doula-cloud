<script lang="ts">
	/*
	 * PROTOTYPE -- #1502, variant D: variant A's page, but no confirmation
	 * page. govuk-alignment.md departs from confirmation pages ("outcomes
	 * are announced with Notice in place"), so a send returns the person
	 * to `?from=` -- when `match()` recognizes it -- with a status Notice
	 * at the top of the screen they left.
	 */
	import { tick } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import FeedbackForm from './FeedbackForm.svelte';
	import HostBody from './HostBody.svelte';
	import PhaseBanner from './PhaseBanner.svelte';
	import type { Report } from './StatePanel.svelte';
	import { copy, readContext, type HostScreen, type Shell } from './fixtures.js';

	interface Properties {
		shell: Shell;
		screen: HostScreen;
		routeId: string;
		onReport: (report: Report) => void;
		onNavigate: (key: string) => void;
	}

	let { shell, screen, routeId, onReport }: Properties = $props();

	const words = $derived(copy[shell]);
	const base = $derived(shell === 'staff' ? '/practices/p_7f3a' : '/portal/engagements/e_55c0');
	const feedbackHref = $derived(`${base}/feedback?from=${encodeURIComponent(screen.path)}`);

	let view = $state<Report['view']>('host');
	let main = $state<HTMLElement>();

	async function show(next: Report['view'], url: string, sent?: Report['sent']) {
		view = next;
		onReport({ view: next, url, sent });
		await tick();
		const target = main?.querySelector<HTMLElement>(next === 'sent' ? '[role="status"]' : 'h1');
		target?.setAttribute('tabindex', '-1');
		target?.focus();
	}
</script>

<PhaseBanner
	text={words.bannerText}
	linkText={words.bannerLink}
	href={feedbackHref}
	onActivate={(event) => {
		event.preventDefault();
		if (view !== 'form') void show('form', feedbackHref);
	}}
/>

<main id="main" tabindex="-1" bind:this={main}>
	{#if view === 'host' || view === 'sent'}
		{#if view === 'sent'}
			<div class="notice">
				<Notice variant="status" message={`${words.confirmationTitle}. ${words.confirmationBody}`} />
			</div>
		{/if}
		<HostBody screenKey={screen.key} title={screen.title} />
	{:else if view === 'form'}
		<div class="column">
			<a
				class="back"
				href={screen.path}
				onclick={(event) => {
					event.preventDefault();
					void show('host', '');
				}}>← Back</a
			>
			<Heading level={1} text={words.pageTitle} />
			<Text text={words.intro} measure />
			<FeedbackForm
				{shell}
				context={readContext(shell, screen, routeId)}
				onSent={({ kind, text }) =>
					void show('sent', screen.path, {
						kind,
						text,
						context: readContext(shell, screen, routeId)
					})}
			/>
		</div>
	{/if}
</main>

<style>
	main {
		max-inline-size: 72rem;
		margin-inline: auto;
		padding: var(--space-6) var(--space-4) var(--space-12);
	}

	.column {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		max-inline-size: 40rem;
	}

	.notice {
		margin-block-end: var(--space-5);
	}


</style>
