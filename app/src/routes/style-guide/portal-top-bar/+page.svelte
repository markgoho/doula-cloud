<script lang="ts">
	import PortalTopBar from '#lib/components/organisms/PortalTopBar.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	const navItems = [
		{ label: 'Your care', href: '#care', current: true },
		{ label: 'Messages', href: '#messages', current: false },
		{ label: 'Birth plan', href: '#birth-plan', current: false },
		{ label: 'Contract', href: '#contract', current: false }
	];

	function signOut(): Promise<SignOutOutcome> {
		return Promise.resolve({ ok: true });
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Portal top bar</h1>

	<p>
		Deliberately not the Staff answer. The Practice's name is the portal's identity rather than
		<code>Doula Cloud</code>, because a Client's relationship is with her doula's practice and not
		with the software it runs on; there is no Practice switcher, because a Client belongs to exactly
		one Practice. Narrow, the four items become a full-width second row rather than a hamburger —
		four items need no container.
	</p>

	<PortalTopBar
		practiceName="Riverside Doula Collective"
		{navItems}
		name="Tasha Bell"
		{signOut}
	/>
</stack-l>
