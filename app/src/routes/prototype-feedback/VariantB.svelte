<script lang="ts">
	/*
	 * PROTOTYPE -- #1502, variant B: the banner link opens a modal
	 * <dialog> over the screen the person is on. The origin needs no
	 * carrying -- it is `page.url` and `page.route.id` of the screen still
	 * underneath -- and the way back is closing the dialog. Needs JS; a
	 * new-tab open or a reload loses the form.
	 */
	import { tick } from 'svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Dialog from '#lib/components/molecules/Dialog.svelte';
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

	let open = $state(false);
	let isSent = $state(false);
	let body = $state<HTMLElement>();

	$effect(() => {
		if (!open && isSent) isSent = false;
		if (!isSent) onReport({ view: open ? 'form' : 'host', url: '' });
	});

	async function focusHeading() {
		await tick();
		const heading = body?.querySelector('h2');
		heading?.setAttribute('tabindex', '-1');
		heading?.focus();
	}
</script>

<PhaseBanner
	text={words.bannerText}
	linkText={words.bannerLink}
	href="#feedback"
	onActivate={(event) => {
		event.preventDefault();
		open = true;
	}}
/>

<main id="main" tabindex="-1">
	<HostBody screenKey={screen.key} title={screen.title} />
</main>

<div>
<Dialog bind:open label={isSent ? words.confirmationTitle : words.pageTitle}>
	<div class="body" bind:this={body}>
		{#if isSent}
			<Heading level={2} text={words.confirmationTitle} />
			<Text text={words.confirmationBody} />
			<Button label={`Return to ${screen.title}`} onClick={() => (open = false)} />
		{:else}
			<Heading level={2} text={words.pageTitle} />
			<Text text={words.intro} />
			<FeedbackForm
				{shell}
				context={readContext(shell, screen, routeId)}
				onSent={({ kind, text }) => {
					isSent = true;
					onReport({ view: 'sent', url: '', sent: { kind, text, context: readContext(shell, screen, routeId) } });
					void focusHeading();
				}}
			/>
			<Button label="Cancel" variant="secondary" onClick={() => (open = false)} />
		{/if}
	</div>
</Dialog>
</div>

<style>
	main {
		max-inline-size: 72rem;
		margin-inline: auto;
		padding: var(--space-6) var(--space-4) var(--space-12);
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		max-inline-size: 36rem;
	}
</style>
