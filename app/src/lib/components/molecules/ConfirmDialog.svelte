<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import Dialog from './Dialog.svelte';

	/*
	 * The one confirmation mechanism for every destructive/irreversible
	 * action in the app (#473). Deliberately no typed-name confirmation and
	 * no generic "OK" -- the confirm button always names the action it
	 * takes, so a person reads what they are about to do rather than
	 * dismissing a familiar shape.
	 *
	 * `error` is the one addition #804 makes: a rejected `onConfirm` leaves
	 * the dialog open (below), and the native `<dialog>` this renders on
	 * (`Dialog.svelte`) sits in the browser's top layer above a `::backdrop`
	 * -- so a caller's failure has to be rendered in here to stay readable
	 * while the dialog is still open, not in the page behind it. This is a
	 * control's own operation outcome, not a refused form (#467's own
	 * distinction), so it renders as `Notice`, matching every other
	 * section-local outcome in the app rather than growing an `ErrorSummary`.
	 */
	interface Properties {
		open?: boolean;
		title: string;
		consequence: string;
		confirmLabel: string;
		error?: string;
		onConfirm: () => void | Promise<void>;
		onCancel?: () => void;
	}

	let {
		open = $bindable(false),
		title,
		consequence,
		confirmLabel,
		error,
		onConfirm,
		onCancel
	}: Properties = $props();

	let isConfirming = $state(false);

	async function handleConfirm() {
		isConfirming = true;
		try {
			await onConfirm();
			open = false;
		} catch {
			// Left open on purpose -- the caller sets `error` in response to
			// the rejection, and this component renders it above (#804).
		} finally {
			isConfirming = false;
		}
	}

	function handleCancel() {
		open = false;
		onCancel?.();
	}
</script>

<Dialog bind:open label={title}>
	<div class="content">
		<Heading level={2} text={title} />
		<WarningText message={consequence} />
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
		<div class="actions">
			<Button label="Cancel" variant="secondary" onClick={handleCancel} />
			<Button
				label={confirmLabel}
				variant="destructive"
				loading={isConfirming}
				onClick={handleConfirm}
			/>
		</div>
	</div>
</Dialog>

<style>
	@layer components {
		.content {
			display: flex;
			flex-direction: column;
			gap: var(--space-4);
		}

		.actions {
			display: flex;
			justify-content: flex-end;
			gap: var(--space-3);
		}
	}
</style>
