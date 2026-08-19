<script lang="ts">
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';
	import { SIGN_OUT_FAILED_MESSAGE, type SignOutOutcome } from '#lib/signOut.js';

	// The style guide never really signs anyone out -- each state below is
	// driven by a stub the component treats exactly like the real thing.
	const succeeds = async (): Promise<SignOutOutcome> => ({ ok: true });
	const fails = async (): Promise<SignOutOutcome> => ({
		ok: false,
		message: SIGN_OUT_FAILED_MESSAGE
	});
	const neverSettles = () => new Promise<SignOutOutcome>(() => {});
</script>

<stack-l space="var(--space-6)">
	<h1>Sign out button</h1>

	<section>
		<h2>Default</h2>
		<SignOutButton signOut={succeeds} />
	</section>

	<section>
		<h2>Signing out</h2>
		<p>Click to leave the button in its in-flight state; it stays there.</p>
		<SignOutButton signOut={neverSettles} />
	</section>

	<section>
		<h2>Failed</h2>
		<p>Click to see what a sign-out that did not go through reports.</p>
		<SignOutButton signOut={fails} />
	</section>
</stack-l>
