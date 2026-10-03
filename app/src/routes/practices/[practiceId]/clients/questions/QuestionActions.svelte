<script lang="ts">
	/*
	 * What a question page offers under its question: go on, and -- in
	 * intake -- stop here and keep what has been said.
	 *
	 * Extracted because it is six identical consumers, not two -- ADR-0018's
	 * bar, met several times over. It lives beside the routes rather than
	 * in the design system because the second button is not a generic
	 * "cancel": it is ADR-0017's free save, which only means anything
	 * inside a sequence that has a record still to save. A Client's details
	 * journey (#1610) saves once, at its check page, so it passes no
	 * `onSaveForLater` and shows only Continue.
	 *
	 * `Continue` is the submit, so the form it sits in submits on Enter
	 * from any field -- GOV.UK's implicit submission, and the reason each
	 * route wraps its `QuestionPage` in a real <form>.
	 */
	import Button from '#lib/components/atoms/Button.svelte';

	interface Properties {
		/**
		 * Named after what comes next where there is a next thing to name.
		 * Falls back to GOV.UK's own word.
		 */
		continueLabel?: string;
		isSaving?: boolean;
		onSaveForLater?: () => void;
	}

	let { continueLabel = 'Continue', isSaving = false, onSaveForLater }: Properties = $props();
</script>

<Button type="submit" label={continueLabel} />
{#if onSaveForLater}
	<Button
		variant="secondary"
		label="Save and come back later"
		loading={isSaving}
		onClick={onSaveForLater}
	/>
{/if}
