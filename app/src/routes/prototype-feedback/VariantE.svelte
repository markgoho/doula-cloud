<script lang="ts">
	/*
	 * PROTOTYPE -- #1502, variant E: the banner link opens a drawer at the
	 * inline end. Not modal: the screen beside it stays visible, scrolls,
	 * and can be read while typing about it. The origin is the screen
	 * underneath, as in B. Escape or Close shuts it and returns focus to
	 * the banner link. At 320px the drawer is the full width.
	 */
	import { tick } from 'svelte';
	import Button from '#lib/components/atoms/Button.svelte';
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

	let isOpen = $state(false);
	let isSent = $state(false);
	let drawer = $state<HTMLElement>();
	let opener: HTMLElement | undefined;

	async function open(event: MouseEvent) {
		event.preventDefault();
		opener = event.currentTarget as HTMLElement;
		isOpen = true;
		onReport({ view: 'form', url: '' });
		await tick();
		focusHeading();
	}

	function focusHeading() {
		const heading = drawer?.querySelector<HTMLElement>('h2');
		heading?.setAttribute('tabindex', '-1');
		heading?.focus();
	}

	function close() {
		isOpen = false;
		if (!isSent) onReport({ view: 'host', url: '' });
		opener?.focus();
	}

	function onKeydown(event: KeyboardEvent) {
		if (isOpen && event.key === 'Escape') close();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<PhaseBanner text={words.bannerText} linkText={words.bannerLink} href="#feedback" onActivate={open} />

<div class="layout"><div class="row">
<main id="main" tabindex="-1">
	{#if isSent && !isOpen}
		<div class="notice">
			<Notice variant="status" message={`${words.confirmationTitle}. ${words.confirmationBody}`} />
		</div>
	{/if}
	<HostBody screenKey={screen.key} title={screen.title} />
</main>

{#if isOpen}
	<aside class="drawer" bind:this={drawer} aria-labelledby="feedback-drawer-heading">
		<div class="head">
			<Heading level={2} id="feedback-drawer-heading" text={words.pageTitle} />
			<Button label="Close" variant="secondary" size="sm" onClick={close} />
		</div>
		<Text text={words.intro} />
			<FeedbackForm
				{shell}
				context={readContext(shell, screen, routeId)}
				onSent={async ({ kind, text }) => {
					isSent = true;
					isOpen = false;
					onReport({ view: 'sent', url: '', sent: { kind, text, context: readContext(shell, screen, routeId) } });
					await tick();
					// Outcomes are a Notice in place (govuk-alignment.md): the
					// drawer closes and the screen it sat beside says so.
					const notice = document.querySelector<HTMLElement>('main [role="status"]');
					notice?.setAttribute('tabindex', '-1');
					notice?.focus();
				}}
			/>
	</aside>
{/if}
</div></div>

<style>
	/* The drawer sits beside the screen and pushes it, never over it, while
	   the row has room for both; below that it takes the whole width. */
	.layout {
		container-type: inline-size;
	}

	.row {
		display: flex;
		align-items: flex-start;
	}

	main {
		flex: 1 1 0;
		min-inline-size: 0;
		max-inline-size: 72rem;
		margin-inline: auto;
		padding: var(--space-6) var(--space-4) var(--space-12);
	}

	.notice {
		margin-block-end: var(--space-5);
	}

	.drawer {
		position: sticky;
		inset-block-start: 0;
		flex: 0 0 28rem;
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		max-block-size: 100dvh;
		overflow-y: auto;
		padding: var(--space-5) var(--space-4) var(--space-12);
		border-inline-start: var(--border-thin) solid var(--color-outline-variant);
		background: var(--color-surface-container);
	}

	@container (inline-size < 52rem) {
		.drawer {
			position: fixed;
			inset: 0;
			z-index: 10;
			max-block-size: none;
		}
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
	}
</style>
