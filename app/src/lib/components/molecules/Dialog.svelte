<script lang="ts">
	import type { Snippet } from 'svelte';

	/*
	 * The generic native-<dialog> primitive (#473): showModal()/close() so
	 * the top layer, light dismiss, Escape and focus return are the
	 * browser's job, never hand-rolled again. StaffTopBar's sheet is the
	 * one existing hand-written version of this; new call sites use this
	 * instead.
	 */
	interface Properties {
		open?: boolean;
		label: string;
		children: Snippet;
	}

	let { open = $bindable(false), label, children }: Properties = $props();

	let dialog = $state<HTMLDialogElement>();

	$effect(() => {
		/* v8 ignore next -- dialog is bound via bind:this before this effect
		   ever runs; the guard exists only to satisfy $state<HTMLDialogElement>()'s
		   optional type, not a reachable branch. */
		if (!dialog) return;
		if (open && !dialog.open) {
			dialog.showModal();
		} else if (!open && dialog.open) {
			dialog.close();
		}
	});

	// Fires on Escape and on any close() call, so this is the one place
	// that needs to sync `open` back -- callers never listen for Escape.
	function handleClose() {
		open = false;
	}
</script>

<dialog bind:this={dialog} aria-label={label} onclose={handleClose}>
	{@render children()}
</dialog>

<style>
	@layer components {
		dialog {
			max-inline-size: min(32rem, calc(100dvw - var(--space-8)));
			padding: var(--space-6);
			border: 0;
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface);
		}

		dialog::backdrop {
			background-color: color-mix(in oklch, var(--color-on-surface) 50%, transparent);
		}
	}
</style>
