<script lang="ts">
	import Button from './components/atoms/Button.svelte';

	/**
	 * Shared status display for a Contract: the current status, a clear
	 * terminal-state indicator once Voided, and (when the caller is Staff)
	 * the Void action itself -- offered only on a Signed Contract, since
	 * Void is a one-way transition into the terminal Voided state. Used by
	 * both the Staff Engagement view and the Client-portal Contract view;
	 * the Client-portal caller omits onVoid, so the button never renders
	 * there.
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
