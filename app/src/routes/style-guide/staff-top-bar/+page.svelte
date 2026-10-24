<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		hasSections?: boolean;
	}

	/*
	 * #1837: a screen that offers no section -- the agreement screen while
	 * an agreement is due (#1549), and /account -- hands the bar an empty
	 * list, and the bar draws no nav and no Menu button for it.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Staff top bar, on a screen that offers no section', props: { hasSections: false } }
	];
</script>

<script lang="ts">
	import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
	import type { PracticeOption } from '#lib/components/molecules/PracticeSwitcher.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	let { hasSections = true }: Properties = $props();

	/*
	 * The longest realistic value, not a representative one (ADR-0025):
	 * the nav an Owner or an Admin sees, every section
	 * `practices/+layout.svelte` offers, in its order. This demo held six
	 * long after the app offered ten, so the bar's floor was measured on a
	 * nav nobody with the admin sections had, and a 1280px window scrolled
	 * sideways (#1772). A Doula's shorter list is the same tree with fewer
	 * links in it, so it is not a variant of its own.
	 */
	const allSections = [
		{ label: 'Overview', href: '#overview', current: true },
		{ label: 'Clients', href: '#clients', current: false },
		{ label: 'Schedule', href: '#schedule', current: false },
		{ label: 'On call', href: '#on-call', current: false },
		{ label: 'Invoices', href: '#invoices', current: false },
		{ label: 'Contracts', href: '#contracts', current: false },
		{ label: 'Credits', href: '#credits', current: false },
		{ label: 'Staff', href: '#staff', current: false },
		{ label: 'Offers', href: '#offers', current: false },
		{ label: 'Settings', href: '#settings', current: false }
	];
	const navItems = $derived(hasSections ? allSections : []);

	/*
	 * A Practice registers under its legal name and a Doula's address is on
	 * the Practice's domain -- those are what the bar has to fit too.
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
		Two rows: the lockup, the Practice switcher and the avatar on a 60px band that grows when a
		long Practice name wraps, and the sections on a row of their own under it. Narrow the bar past
		57rem and the nav and the Practice switcher move into a full-screen sheet behind a
		hamburger — a bottom tab bar was drawn and rejected, because five slots cannot carry ten
		sections without a <code>More</code>, and <code>More</code> is not a noun this domain has. A
		screen that offers no section draws no nav and no hamburger.
	</p>

	<StaffTopBar
		{navItems}
		{practices}
		currentPracticeId="p1"
		firstName="Persephone"
		lastName="Adeyemi-Wollstonecraft"
		email="persephone.adeyemi-wollstonecraft@highland-midwifery-group.example.org"
		accountHref="#account"
		{signOut}
	/>
</stack-l>
