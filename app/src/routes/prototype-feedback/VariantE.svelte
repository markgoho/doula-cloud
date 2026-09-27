<script lang="ts">
	/*
	 * PROTOTYPE -- #1502, variant E: the banner link slides a drawer in
	 * from the inline end, over the screen, which does not move. It slides
	 * back out on Close, Escape, or a send. The origin is the screen
	 * underneath, as in B. Focus goes to the drawer's heading, and back to
	 * the banner link on close. Motion stops under prefers-reduced-motion.
	 * At 320px the drawer is the full width.
	 */
	import { tick } from 'svelte';
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
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

	const isReducedMotion = globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
	// Slides its own width: `x` in px, so read the drawer's rendered width.
	const slide = (node: HTMLElement) =>
		fly(node, { x: node.offsetWidth, opacity: 1, duration: isReducedMotion ? 0 : 240, easing: cubicOut });

	async function open(event: MouseEvent) {
		event.preventDefault();
		opener = event.currentTarget as HTMLElement;
		isOpen = true;
		onReport({ view: 'form', url: '' });
		await tick();
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

<main id="main" tabindex="-1">
	{#if isSent && !isOpen}
		<div class="notice">
			<Notice variant="status" message={`${words.confirmationTitle}. ${words.confirmationBody}`} />
		</div>
	{/if}
	<HostBody screenKey={screen.key} title={screen.title} />
</main>

{#if isOpen}
	<aside class="drawer" bind:this={drawer} aria-labelledby="feedback-drawer-heading" transition:slide>
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
				// drawer slides away and the screen under it says so.
				const notice = document.querySelector<HTMLElement>('main [role="status"]');
				notice?.setAttribute('tabindex', '-1');
				notice?.focus();
			}}
		/>
	</aside>
{/if}

<style>
	main {
		max-inline-size: 72rem;
		margin-inline: auto;
		padding: var(--space-6) var(--space-4) var(--space-12);
	}

	.notice {
		margin-block-end: var(--space-5);
	}

	/* Over the screen, never beside it: the screen keeps its layout. */
	.drawer {
		position: fixed;
		inset-block: 0;
		inset-inline-end: 0;
		z-index: 10;
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		inline-size: min(28rem, 100%);
		overflow-y: auto;
		padding: var(--space-5) var(--space-4) var(--space-12);
		border-inline-start: var(--border-thin) solid var(--color-outline-variant);
		background: var(--color-surface-container);
		box-shadow: -8px 0 24px rgb(0 0 0 / 25%);
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
	}
</style>
