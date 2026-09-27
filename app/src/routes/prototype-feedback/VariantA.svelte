<script lang="ts">
	/*
	 * PROTOTYPE -- #1502, variant A: GOV.UK's own shape. The banner link is
	 * a plain <a> to a feedback page of its own, carrying the origin as
	 * `?from=<path>`. The feedback page resolves the route pattern with
	 * `match(from)`, and only a path that matches an app route earns a
	 * "Return to" link -- anything else falls back to the landing page, so
	 * `?from=` is never an open redirect. Works with no JS, in a new tab,
	 * and after a reload.
	 */
	import { tick } from 'svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
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
		const heading = main?.querySelector('h1');
		heading?.setAttribute('tabindex', '-1');
		heading?.focus();
	}
</script>

<PhaseBanner
	text={words.bannerText}
	linkText={words.bannerLink}
	href={feedbackHref}
	onActivate={(event) => {
		event.preventDefault();
		if (view === 'host') void show('form', feedbackHref);
	}}
/>

<main id="main" tabindex="-1" bind:this={main}>
	{#if view === 'host'}
		<HostBody screenKey={screen.key} title={screen.title} />
	{:else if view === 'form'}
		<div class="column">
			<BackLink href={screen.path} />
			<Heading level={1} text={words.pageTitle} />
			<Text text={words.intro} measure />
			<FeedbackForm
				{shell}
				context={readContext(shell, screen, routeId)}
				onSent={({ kind, text }) =>
					void show('sent', `${base}/feedback/sent?from=${encodeURIComponent(screen.path)}`, {
						kind,
						text,
						context: readContext(shell, screen, routeId)
					})}
			/>
		</div>
	{:else}
		<div class="column">
			<div class="panel">
				<Heading level={1} text={words.confirmationTitle} />
			</div>
			<Text text={words.confirmationBody} measure />
			<a
				href={screen.path}
				onclick={(event) => {
					event.preventDefault();
					void show('host', '');
				}}>Return to {screen.title}</a
			>
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

	.panel {
		padding: var(--space-5) var(--space-4);
		border-radius: var(--radius);
		background: var(--color-status);
		color: var(--color-on-primary);
	}

	.panel :global(h1) {
		color: inherit;
	}

	a {
		color: var(--color-primary);
	}
</style>
