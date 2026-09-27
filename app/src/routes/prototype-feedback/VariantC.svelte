<script lang="ts">
	/*
	 * PROTOTYPE -- #1502, variant C: GOV.UK's "Report a problem with this
	 * page" shape. Every screen ends with a closed <details> holding the
	 * form; the banner link is `#feedback` and opens it. The origin is the
	 * screen the form sits on, and there is no way back to find because
	 * the person never left. The <details> opens with no JS.
	 */
	import { tick } from 'svelte';
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

	let isOpen = $state(false);
	let isSent = $state(false);
	let section = $state<HTMLElement>();

	$effect(() => {
		if (!isSent) onReport({ view: isOpen ? 'form' : 'host', url: isOpen ? `${screen.path}#feedback` : '' });
	});

	async function openAndFocus() {
		isOpen = true;
		await tick();
		section?.scrollIntoView({ block: 'start' });
		section?.querySelector('summary')?.focus();
	}
</script>

<PhaseBanner
	text={words.bannerText}
	linkText={words.bannerLink}
	href="#feedback"
	onActivate={(event) => {
		event.preventDefault();
		void openAndFocus();
	}}
/>

<main id="main" tabindex="-1">
	<HostBody screenKey={screen.key} title={screen.title} />

	<section id="feedback" class="feedback" bind:this={section} aria-labelledby="feedback-heading">
		{#if isSent}
			<Heading level={2} id="feedback-heading" text={words.confirmationTitle} />
			<Text text={words.confirmationBody} measure />
		{:else}
			<details bind:open={isOpen}>
				<summary><span id="feedback-heading">{words.bannerLink}</span></summary>
				<div class="body">
					<Text text={words.intro} measure />
					<FeedbackForm
						{shell}
						context={readContext(shell, screen, routeId)}
						onSent={async ({ kind, text }) => {
							isSent = true;
							onReport({ view: 'sent', url: `${screen.path}#feedback`, sent: { kind, text, context: readContext(shell, screen, routeId) } });
							await tick();
							const heading = section?.querySelector('h2');
							heading?.setAttribute('tabindex', '-1');
							heading?.focus();
						}}
					/>
				</div>
			</details>
		{/if}
	</section>
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: var(--space-8);
		max-inline-size: 72rem;
		margin-inline: auto;
		padding: var(--space-6) var(--space-4) var(--space-12);
	}

	.feedback {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		max-inline-size: 40rem;
		padding-block-start: var(--space-5);
		border-block-start: var(--border-active) solid var(--color-primary);
	}

	summary {
		color: var(--color-primary);
		text-decoration: underline;
		cursor: pointer;
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		padding-block-start: var(--space-4);
	}
</style>
