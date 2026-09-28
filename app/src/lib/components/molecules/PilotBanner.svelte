<script lang="ts">
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';

	/*
	 * The GOV.UK phase banner shape (#1522):
	 * https://design-system.service.gov.uk/components/phase-banner/
	 * A "Pilot" tag and one sentence, with a control inline in the sentence
	 * that opens the Feedback drawer (#1521).
	 *
	 * `atoms/Badge.svelte` is the tag: `docs/design/govuk-alignment.md`
	 * already maps Badge onto GOV.UK's own Tag component ("Aligned; walked
	 * 2026-08-30"), so this reuses the tag every other status in the app
	 * already draws rather than building a second one to match a picture.
	 *
	 * This molecule owns no copy of its own. The Staff and Portal shells
	 * carry their own sentence and control text (#1502 Q4, #1527, #1528),
	 * because the two shells' sentences differ and neither is more
	 * canonical than the other.
	 *
	 * It owns no drawer state either. `open` mirrors whatever the caller's
	 * Drawer (#1521) is actually doing, so `aria-expanded` never drifts
	 * from the drawer's real state; `controlsId` names that same drawer for
	 * `aria-controls`. `onOpenFeedback` only asks the caller to open it --
	 * closing is the drawer's own affordance (Close, Escape, a send), never
	 * this button's, so there is nothing here to toggle back.
	 *
	 * The control is a `<button>`, not an `<a href="#">`: the resolution on
	 * #1502 states it plainly -- "It goes nowhere, so it is a `<button>`
	 * that looks like the link in the GOV.UK banner." That is
	 * `atoms/Button.svelte`'s new `link` variant: a plain `<button>` is
	 * off limits here (`eslint.config.js`'s `svelte/no-restricted-html-
	 * elements` requires the atom everywhere but the atom itself), and the
	 * variant reads as inline prose rather than as a control with its own
	 * chrome.
	 */
	interface Properties {
		sentence: string;
		controlText: string;
		open: boolean;
		controlsId: string;
		onOpenFeedback: () => void;
	}

	let { sentence, controlText, open, controlsId, onOpenFeedback }: Properties = $props();
</script>

<div class="banner">
	<p>
		<Badge variant="info" label="Pilot" />
		<span
			>{sentence}
			<Button
				variant="link"
				label={controlText}
				expanded={open}
				ariaControls={controlsId}
				onClick={onOpenFeedback}
			/>.</span
		>
	</p>
</div>

<style>
	@layer components {
		.banner {
			padding-block: var(--space-2);
			padding-inline: var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface);
		}

		p {
			display: flex;
			flex-wrap: wrap;
			align-items: baseline;
			gap: var(--space-1) var(--space-3);
			margin: 0;
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
			color: var(--color-on-surface);
		}

		/*
		 * The sentence and its inline control, as one shrinkable flex item
		 * (ADR-0024): a flex item's automatic minimum size is its
		 * min-content size, and prose has spaces to wrap at, so this needs
		 * no `overflow-wrap` override the way `Link.svelte`'s bare-URL case
		 * does -- only `min-inline-size: 0` to let it shrink past that
		 * min-content width once the Pilot tag has taken its own room.
		 */
		span {
			flex: 1 1 16rem;
			min-inline-size: 0;
		}
	}
</style>
