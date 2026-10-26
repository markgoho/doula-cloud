<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	interface Properties {
		/**
		 * Ends the session and takes the person where they belong next --
		 * injected rather than imported so this stays a presentation
		 * component with no route, navigation or fetch knowledge of its own.
		 * Resolves with what to tell the person if sign-out did not go
		 * through.
		 */
		signOut: () => Promise<SignOutOutcome>;
		/*
		 * `menu`: a row in the avatar menu's panel (#2141), at the weight of
		 * the Account row above it. `secondary` is the button a screen with
		 * no menu draws, the signed-out pages' own way out.
		 */
		variant?: 'secondary' | 'menu';
	}

	let { signOut, variant = 'secondary' }: Properties = $props();

	let isSigningOut = $state(false);
	let error = $state('');

	async function handleClick() {
		error = '';
		// Button renders `loading` as disabled, so this is also what makes a
		// double-click harmless on the client side; the end-session endpoint
		// is idempotent, which covers the stale-tab case (#152).
		isSigningOut = true;
		const outcome = await signOut();
		isSigningOut = false;
		if (!outcome.ok) error = outcome.message;
	}
</script>

<Button label="Sign out" {variant} size="sm" loading={isSigningOut} onClick={handleClick} />
{#if error}
	<Notice message={error} variant="error" />
{/if}
