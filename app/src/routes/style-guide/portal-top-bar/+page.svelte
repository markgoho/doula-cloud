<script lang="ts">
	import PortalTopBar from '#lib/components/organisms/PortalTopBar.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * nav labels are the product's own and cannot grow, but the Practice
	 * name and the Client's name are hers, and they are what decide whether
	 * the items still fit beside them. All five items, the most the
	 * authenticated layout ever passes (#1568): the floor below is
	 * measured against this demo, so a demo with fewer items measures a
	 * floor the real bar cannot keep.
	 */
	const navItems = [
		{ label: 'Your care', href: '#care', current: true },
		{ label: 'Messages', href: '#messages', current: false },
		{ label: 'Birth plan', href: '#birth-plan', current: false },
		{ label: 'Contract', href: '#contract', current: false },
		{ label: 'Notifications', href: '#notifications', current: false }
	];

	function signOut(): Promise<SignOutOutcome> {
		return Promise.resolve({ ok: true });
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Portal top bar</h1>

	<p>
		Deliberately not the Staff answer. The Practice's name is the portal's identity rather than
		<code>Doula Cloud</code>, because a Client's relationship is with the Client's doula practice and
		not with the software it runs on; there is no Practice switcher dropdown, because a Client's own
		Client record belongs to exactly one Practice. The Practice name is instead a plain link to the
		portal root, the Client's way between Engagements when the Client's Portal Account reaches more
		than one. Narrow, the items become a full-width second row rather than a hamburger, and that row
		wraps when they do not fit on one line — a handful of items need no container.
	</p>

	<PortalTopBar
		practiceName="Highland Midwifery &amp; Birth Support Collective of Western New York"
		switcherLabel="Highland Midwifery &amp; Birth Support Collective of Western New York, started Mar 12, 2026"
		{navItems}
		name="Anne-Marie Ochieng-Whitfield"
		accountHref="#sign-in-address"
		{signOut}
	/>
</stack-l>
