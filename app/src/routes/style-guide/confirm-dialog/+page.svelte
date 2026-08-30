<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';

	// The style guide never really removes anyone -- each demo below drives
	// the component with a stub it treats exactly like the real thing.
	const succeeds = async () => {};
	const fails = async () => {
		throw new Error('demo failure');
	};

	let isOpenSucceeds = $state(false);
	let isOpenFails = $state(false);
</script>

<stack-l space="var(--space-6)">
	<h1>Confirm dialog</h1>

	<section>
		<h2>Default</h2>
		<p>
			The one confirmation mechanism for a destructive action: a stated consequence, a
			<code>Cancel</code>, and a confirm button whose label names the action rather than a generic
			"OK".
		</p>
		<Button
			label="Remove from Practice"
			variant="destructive"
			onClick={() => (isOpenSucceeds = true)}
		/>
		<ConfirmDialog
			bind:open={isOpenSucceeds}
			title="Remove from Practice"
			consequence="This removes the Doula from the Practice. This cannot be undone."
			confirmLabel="Remove from Practice"
			onConfirm={succeeds}
		/>
	</section>

	<section>
		<h2>A failing confirm</h2>
		<p>Click through to see the dialog stay open when <code>onConfirm</code> rejects.</p>
		<Button
			label="Decline this offer"
			variant="destructive"
			onClick={() => (isOpenFails = true)}
		/>
		<ConfirmDialog
			bind:open={isOpenFails}
			title="Decline this offer"
			consequence="This offer cannot be reinstated once declined."
			confirmLabel="Decline this offer"
			onConfirm={fails}
		/>
	</section>
</stack-l>
