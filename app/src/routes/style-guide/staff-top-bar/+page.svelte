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

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * nav labels are the product's own and cannot grow, but a Practice
	 * registers under its legal name and a Doula's address is on the
	 * Practice's domain -- those are what the bar has to fit.
	 */
	const practices: PracticeOption[] = [
		{
			practiceId: 'p1',
			practiceName: 'Highland Midwifery & Birth Support Collective of Western New York',
			roles: ['owner', 'admin'],
			href: '#riverside'
		},
		{
			practiceId: 'p2',
			practiceName: 'Finger Lakes Birth Support and Postpartum Care Cooperative',
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
		name="Persephone Adeyemi-Wollstonecraft"
		email="persephone.adeyemi-wollstonecraft@highland-midwifery-group.example.org"
		accountHref="#account"
		{signOut}
	/>
</stack-l>
