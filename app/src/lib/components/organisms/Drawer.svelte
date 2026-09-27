<script lang="ts">
	import type { Snippet } from 'svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import { syncDialogOpen } from '#lib/components/nativeDialog.js';

	/*
	 * A panel that slides in over the screen from the inline end (#1521),
	 * first built for the pilot Feedback form (#1519) but owning the
	 * behavior on its own -- the form is a consumer, not its definition.
	 *
	 * Built on a native <dialog> the way `molecules/Dialog.svelte` (#473)
	 * is, sharing that component's open/close sync (`nativeDialog.ts`) so
	 * there is one hand-rolled guard against a hidden duplicate's
	 * showModal() (#508) rather than two. What Drawer adds on top: it
	 * picks `showModal()` or `show()` depending on how much room it has,
	 * because those are the two different browser behaviors the AC asks
	 * for -- an inert, focus-trapped background at the full width, and a
	 * background that stays readable and scrollable once the panel is
	 * narrower than the screen -- and there is no CSS that can choose
	 * between two different JS calls.
	 *
	 * No hand-written focus trap and no hand-written focus return either:
	 * the HTML spec's `close()` steps restore focus to whatever was
	 * focused before `show()`/`showModal()` was called, for both --
	 * verified directly (removing an earlier explicit `opener` variable
	 * changed nothing this component's own spec asserts). `showModal()`
	 * additionally traps focus and closes on Escape for the modal case; a
	 * non-modal `show()` dialog never enters the top layer, so
	 * `handleKeydown` below supplies only the one thing that path is
	 * missing -- Escape closing it.
	 */
	interface Properties {
		open?: boolean;
		/**
		 * Both the dialog's accessible name (as its `aria-label`) and the
		 * visible heading rendered inside it -- the same shape
		 * `ConfirmDialog.svelte` already uses for its title.
		 */
		heading: string;
		children: Snippet;
	}

	let { open = $bindable(false), heading, children }: Properties = $props();

	let dialog = $state<HTMLDialogElement>();

	// The same 28rem the stylesheet below sizes the panel with
	// (`inline-size: min(28rem, 100%)`): below it, `min()` resolves to
	// 100% and the panel covers the whole screen, so that is exactly when
	// it must behave as a modal.
	const FULL_WIDTH_QUERY = '(width < 28rem)';

	$effect(() => {
		syncDialogOpen(dialog, open, (d) => {
			if (matchMedia(FULL_WIDTH_QUERY).matches) {
				d.showModal();
			} else {
				d.show();
			}
			/*
			 * `showModal()` would otherwise focus the first focusable
			 * descendant (the Close button) rather than announce the
			 * dialog's own name first, and `show()` moves focus nowhere
			 * at all -- neither is what the reviewed prototype does
			 * ("focus goes to the drawer's heading", #1502). The dialog
			 * itself carries that name (`aria-label`), so focusing it
			 * (via the `tabindex="-1"` below) announces it, and a
			 * keyboard user starts inside the panel rather than on the
			 * trigger behind it.
			 */
			d.focus();
		});
	});

	// Fires on Escape, on light dismiss, and on any close() call -- the one
	// place that needs to sync `open` back, whichever of those closed the
	// dialog.
	function handleClose() {
		open = false;
	}

	// The browser's own Escape handling only exists for a dialog in the top
	// layer (`showModal()`). A non-modal one (`show()`) is a plain element
	// with no such handling, so this supplies exactly the part that is
	// missing rather than duplicating what `:modal` already does for free.
	//
	// On the window, not on the dialog: the non-modal case leaves the
	// uncovered screen interactive, so focus can be out there when Escape
	// is pressed, and a keydown there never bubbles through the dialog.
	function handleKeydown(event: KeyboardEvent) {
		if (dialog?.open && event.key === 'Escape' && !dialog.matches(':modal')) {
			open = false;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<dialog bind:this={dialog} aria-label={heading} tabindex="-1" onclose={handleClose}>
	<div class="head">
		<Heading level={2} text={heading} />
		<Button label="Close" variant="secondary" size="sm" onClick={() => (open = false)} />
	</div>
	<div class="body">
		{@render children()}
	</div>
</dialog>

<style>
	@layer components {
		/*
		 * Over the screen, never beside it (AC1): fixed to the viewport
		 * rather than sized by a layout parent, so the screen under it
		 * keeps its own layout and never reflows. 28rem is the AC's own
		 * number (AC2); `FULL_WIDTH_QUERY` above reads the same value so
		 * the two stay in step.
		 *
		 * The UA's own modal caps (`max-inline-size`/`max-block-size`)
		 * have to go the same way `StaffTopBar`'s full-screen sheet
		 * already overrides them, or a wide panel gets clipped back down
		 * to the UA's own guess at a comfortable dialog size.
		 */
		dialog {
			position: fixed;
			inset-block: 0;
			/* The UA stylesheet centers a <dialog> with `inset: 0` (all
			   four sides) plus `margin: auto`; overriding `margin` alone
			   still leaves its own `inset-inline-start: 0` in effect
			   alongside this rule's `inset-inline-end: 0` below, and with
			   both sides and an explicit width set, the CSS spec's
			   over-constrained resolution keeps `inset-inline-start` and
			   drops `inset-inline-end` in a left-to-right document --
			   measured directly: without this the panel pinned to the
			   wrong edge. `auto` here is what actually cedes the position
			   to `inset-inline-end`. */
			inset-inline-start: auto;
			inset-inline-end: 0;
			margin: 0;
			inline-size: min(28rem, 100%);
			block-size: 100%;
			max-inline-size: none;
			max-block-size: none;
			padding: var(--space-5) var(--space-4) var(--space-8);
			overflow-y: auto;
			border: 0;
			border-inline-start: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
			color: var(--color-on-surface);
			/* Off-screen at rest, past its own inline-end edge: `translate`'s
			   percentages resolve against the element's own box, not its
			   container, so 100% clears the panel regardless of how wide it
			   rendered (28rem, or the full screen). */
			translate: 100% 0;
		}

		/*
		 * `display` lives here, not on the bare `dialog` selector above:
		 * an author rule wins over the UA stylesheet's own
		 * `dialog:not([open]) { display: none }` regardless of
		 * specificity, so a `display: flex` declared unconditionally
		 * would force the panel visible even while closed -- measured
		 * directly, that broke every "is it still in the document" check
		 * a closed drawer needs to pass.
		 */
		dialog[open] {
			display: flex;
			flex-direction: column;
			gap: var(--space-4);
			translate: 0 0;
		}

		/*
		 * Motion is opt-in (AC3), the same shape `atoms/Checkbox.svelte`
		 * already uses: with no `prefers-reduced-motion` support at all,
		 * or with it set to `reduce`, there is no `transition` at all and
		 * `translate` above simply snaps between its two values.
		 *
		 * `display` and `overlay` ride the same transition so a closing
		 * dialog stays rendered (and, while modal, stays in the top
		 * layer) for the slide's duration instead of vanishing on the
		 * first frame -- `allow-discrete` is what lets a property with no
		 * in-between values (`display: none` <-> `block`) animate at all.
		 * `overlay` only matters to the modal path (a `show()`d dialog is
		 * never promoted to the top layer), and is a harmless no-op on
		 * the non-modal path the rest of this rule shares with it.
		 */
		@media (prefers-reduced-motion: no-preference) {
			dialog {
				transition:
					translate var(--motion-enter) var(--ease-out),
					display var(--motion-enter) allow-discrete,
					overlay var(--motion-enter) allow-discrete;
			}

			dialog[open] {
				@starting-style {
					translate: 100% 0;
				}
			}
		}

		/* Only ever painted behind the modal (full-width) case -- a
		   non-modal `show()`d dialog has no ::backdrop at all -- and even
		   there it sits behind a panel already covering the whole screen,
		   so this is for the sliver a sub-pixel rounding might leave
		   rather than something a person is expected to see. */
		dialog::backdrop {
			background-color: color-mix(in oklch, var(--color-on-surface) 50%, transparent);
		}

		.head {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			justify-content: space-between;
			gap: var(--space-2);
		}
	}
</style>
