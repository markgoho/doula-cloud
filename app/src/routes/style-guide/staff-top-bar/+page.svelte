<script lang="ts">
	import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
	import type { PracticeOption } from '#lib/components/molecules/PracticeSwitcher.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	const navItems = [
		{ label: 'Overview', href: '#overview', current: true },
		{ label: 'Clients', href: '#clients', current: false },
		{ label: 'Billing', href: '#billing', current: false },
		{ label: 'Staff', href: '#staff', current: false },
		{ label: 'Offers', href: '#offers', current: false },
		{ label: 'Settings', href: '#settings', current: false }
	];

	const practices: PracticeOption[] = [
		{
			practiceId: 'p1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner', 'admin'],
			href: '#riverside'
		},
		{
			practiceId: 'p2',
			practiceName: 'Finger Lakes Birth Support',
			roles: ['doula'],
			href: '#finger-lakes'
		}
	];

	function signOut(): Promise<SignOutOutcome> {
		return Promise.resolve({ ok: true });
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Staff top bar</h1>

	<p>
		A 60px band that does not grow. Narrow the window past 60rem and the nav and the Practice
		switcher move into a full-screen sheet behind a hamburger — a bottom tab bar was drawn and
		rejected, because five slots cannot carry six sections without a <code>More</code>, and
		<code>More</code> is not a noun this domain has.
	</p>

	<StaffTopBar
		{navItems}
		{practices}
		currentPracticeId="p1"
		name="Mark Goho"
		email="markgoho@gmail.com"
		accountHref="#account"
		{signOut}
	/>
</stack-l>
