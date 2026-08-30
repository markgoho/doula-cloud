<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import Dialog from './Dialog.svelte';

	/*
	 * The one confirmation mechanism for every destructive/irreversible
	 * action in the app (#473). Deliberately no typed-name confirmation and
	 * no generic "OK" -- the confirm button always names the action it
	 * takes, so a person reads what they are about to do rather than
	 * dismissing a familiar shape.
	 */
	interface Properties {
		open?: boolean;
		title: string;
		consequence: string;
		confirmLabel: string;
		onConfirm: () => void | Promise<void>;
		onCancel?: () => void;
	}

	let {
		open = $bindable(false),
		title,
		consequence,
		confirmLabel,
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
			// Left open on purpose -- the caller renders its own error state,
			// this component's only job is not closing over a failure.
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
