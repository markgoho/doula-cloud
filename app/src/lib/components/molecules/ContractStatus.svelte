<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';

	/**
	 * Status display for a Contract on the Staff Engagement view: the raw
	 * status, a clear terminal-state indicator once Voided, and the Void
	 * action -- offered only on a Signed Contract, since Void is a one-way
	 * transition into the terminal Voided state. The Client-portal Contract
	 * view stopped using this component on #212 (NH-G5): a Client reads
	 * `clientRegister.ts`'s own label and voided notice, never this
	 * component's Staff wording.
	 */
	let {
		status,
		onVoid
	}: {
		status: string;
		onVoid?: () => Promise<void>;
	} = $props();

	let isVoiding = $state(false);
	let error = $state('');

	// Only ever wired to the Void button below, which itself only renders
	// when onVoid is provided -- the non-null assertion reflects that,
	// rather than adding an unreachable defensive branch.
	async function handleVoid() {
		error = '';
		isVoiding = true;
		try {
			await onVoid!();
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to void contract';
		} finally {
			isVoiding = false;
		}
	}
</script>

<p>Status: {status}</p>

{#if status === 'voided'}
	<p role="status">Voided — this Contract is no longer active.</p>
{/if}

{#if status === 'signed' && onVoid}
	<Button label="Void Contract" onClick={handleVoid} loading={isVoiding} />
{/if}

{#if error}
	<p role="alert">{error}</p>
{/if}
