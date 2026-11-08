<script lang="ts">
	/*
	 * A Continue that may cost something (#610). The warning goes on the
	 * button, not on a screen of its own: the button is already a
	 * deliberate press on a request the BFF handles, and it is the BFF's
	 * refusal of the first press that supplies the words. The label changes
	 * with the warning, so the action names what it does (GOV.UK's rule for
	 * a button that carries a consequence).
	 */
	import Button from '#lib/components/atoms/Button.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';

	interface Properties {
		/**
		What continuing costs, once the BFF has said so; absent until then.
		*/
		warning?: string;
		loading?: boolean;
		onClick: () => void;
	}

	let { warning, loading = false, onClick }: Properties = $props();
</script>

{#if warning}
	<WarningText message={warning} />
{/if}
<Button type="button" label={warning ? 'Continue and sign out' : 'Continue'} {loading} {onClick} />
