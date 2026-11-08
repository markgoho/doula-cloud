<script lang="ts">
	import SignOutConfirmation from '#lib/components/molecules/SignOutConfirmation.svelte';

	// A style-guide control changes nothing beyond itself.
	function ignore() {}
</script>

<stack-l space="var(--space-6)">
	<h1>Sign-out confirmation</h1>

	<p>
		What a signed-out screen shows in place of its form when an action would end a session that is
		already open: the consequence, then the button that acts and the way out.
	</p>

	<stack-l space="var(--space-4)">
		<SignOutConfirmation
			warning="Continuing signs you out of the Practice you are in now. Anything you have not saved there is lost."
			isSubmitting={false}
			onConfirm={ignore}
			onCancel={ignore}
		/>
	</stack-l>
</stack-l>
