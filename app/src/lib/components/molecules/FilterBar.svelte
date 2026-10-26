<script lang="ts">
	/*
	 * The bar that narrows a list (#2026, the #2001 audit's root cause 13).
	 * Schedule and On call each built this form by hand, and the two copies
	 * had drifted: a different legend, a different button label, and the
	 * same fieldset, flex row and summary sentence written twice. Two
	 * consumers that should change together is ADR-0018's bar for a
	 * molecule. Named by structure, not by look.
	 *
	 * GOV.UK, NHS.UK and ONS have no filter component, so this is rule-5
	 * original work (its row in docs/design/govuk-alignment.md); the MOJ
	 * pattern library's filter is the nearest published shape.
	 *
	 * A real `<form method="get">`: Enter submits it, the browser's own
	 * controls do the entry, and the fields are named so the form still
	 * reaches the right URL with no script running at all. `onSubmit` is
	 * the route's upgrade to a client-side navigation, and it owns
	 * `preventDefault()` (Rule of Least Power). That GET submission is why
	 * this is not `StackedForm`, whose fields stack and whose submit is
	 * the page's own.
	 *
	 * The fields sit at their own content width in a cluster that wraps,
	 * never stretched across the column: a date is ten characters, and at
	 * 1440px a stretched one was 600px wide with Apply 1,200px from the
	 * first field. A date control sizes to itself (#1216); a text field or
	 * a select states its `width` on its `LabeledField`. At 320px the
	 * cluster puts one field on each row, with no width query (ADR-0024).
	 *
	 * The result count is the bar's own status line (`role="status"`, a
	 * polite live region), so a new range's result is announced and the
	 * count reads as part of the bar rather than a loose sentence under
	 * it. When the list holds nothing, its `EmptyState` says so on screen,
	 * and the line stays in the document, visually hidden, so it is still
	 * announced (#2015). It is hidden with a class rather than removed, so
	 * the live region exists before its text changes.
	 *
	 * `isAppliedOnChange` (#2068) is the bar whose every control acts the
	 * moment it changes -- the Clients list's "See everyone" toggle. A
	 * switch is a setting that takes effect at once, so an Apply after it
	 * would be a second press for nothing; the bar draws none. The route
	 * navigates from the control's own change. Schedule and On call keep
	 * Apply: a date is typed a character at a time, and a list that
	 * reloaded on each one would move under the person typing it.
	 */
	import type { Snippet } from 'svelte';
	import Button from '#lib/components/atoms/Button.svelte';

	interface Properties {
		/**
		 * The list's own URL: where the form goes with no script running.
		 */
		action: string;
		/**
		 * The route's client-side navigation. It owns `preventDefault()`.
		 * A bar with Apply needs it; an `isAppliedOnChange` bar of controls
		 * that never submit a form (a switch) has nothing to hand it.
		 */
		onSubmit?: (event: SubmitEvent) => void;
		/**
		 * What the bar does, in a few words ("Narrow the schedule").
		 */
		legend: string;
		/**
		 * The result, in words ("Showing 2 scheduled visits.").
		 */
		status: string;
		/**
		 * True when the list under the bar holds nothing: its empty state
		 * says so on screen, and the status line is then for a screen
		 * reader only.
		 */
		empty?: boolean;
		/**
		 * Every control acts the moment it changes (a toggle), so the bar
		 * draws no Apply. The route navigates from the control's change.
		 */
		isAppliedOnChange?: boolean;
		/**
		 * The fields, each a `LabeledField` with a named control.
		 */
		children: Snippet;
	}

	let { action, onSubmit, legend, status, empty = false, isAppliedOnChange = false, children }: Properties = $props();
</script>

<!-- stacked-form:ignore: #2026 -- the one GET narrowing form. Its fields sit side by side and wrap on their own, and `StackedForm` would both stack them and take away the GET submission this form is built on. -->
<form method="get" {action} onsubmit={onSubmit}>
	<fieldset>
		<legend>{legend}</legend>
		<cluster-l space="var(--space-related)" align="flex-end">
			{@render children()}
			{#if !isAppliedOnChange}
				<Button type="submit" label="Apply" variant="secondary" />
			{/if}
		</cluster-l>
	</fieldset>
	<p role="status" class="status" class:visually-hidden={empty}>{status}</p>
</form>

<style>
	@layer components {
		/* The legend, the fields and the status line, at the step that
		   joins related items into one group (#2020). */
		form {
			display: flex;
			flex-direction: column;
			gap: var(--space-related);
		}

		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		/* Quiet: the size and weight of a field's own label, the shape
		   `DateFields` gives its legend, never bold body text, which read
		   as a stray sentence above the fields. `--space-label` to the
		   fields, the step from a question to its control. */
		legend {
			padding: 0;
			margin-block-end: var(--space-label);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		/* A field may shrink below its own width in a column narrower than
		   it, rather than push the page sideways. */
		cluster-l > :global(*) {
			min-inline-size: 0;
		}

		.status {
			margin: 0;
			color: var(--color-on-surface-variant);
			font-size: var(--text-meta-size);
		}
	}
</style>
