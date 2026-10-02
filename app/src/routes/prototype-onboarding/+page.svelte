<script lang="ts">
	/*
	 * PROTOTYPE -- throwaway, wayfinder ticket #1496. The route a new Owner
	 * walks from `/signup` to First Value, as one clickable route with mock
	 * data in memory: sign up, the empty Practice, the Client's name, the
	 * Start work form, the Engagement's page. Built from the real Templates,
	 * the real top bars and the real tokens. Each screen mounts a Template
	 * (EntryPage, OverviewHub, QuestionPage, FormPage, RecordDetail), and
	 * the Template calls PageTitle.
	 *
	 * One variant switch, for one thing: the place of the Credits sentence
	 * on the empty Practice (A, B, C). Each other thing on the route was
	 * decided by a ticket and has no variant.
	 *
	 * Run: `bun run dev` in app/, then open /prototype-onboarding.
	 */
	import { tick } from 'svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';
	import StaffFeedback from '#lib/components/organisms/StaffFeedback.svelte';
	import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
	import ClientScreen from './ClientScreen.svelte';
	import ControlBar from './ControlBar.svelte';
	import EngagementScreen from './EngagementScreen.svelte';
	import NameScreen from './NameScreen.svelte';
	import OverviewScreen from './OverviewScreen.svelte';
	import SignupScreen from './SignupScreen.svelte';
	import StartWorkScreen from './StartWorkScreen.svelte';
	import { nowhere, prototype, to, type Screen } from './model.svelte.js';

	const OUTSIDE: Record<string, string> = {
		details:
			'"Add (name)\'s details" opens the details journey (#1610): the date of birth, the email address, the phone number, the address and the Practice\'s own sections, one per page, then a check page. It is not in this prototype.',
		terms: 'The Terms of Service page does not exist yet (#1505). On the real form the link opens in a new tab.',
		privacy: 'The Privacy Policy page does not exist yet (#1505). On the real form the link opens in a new tab.'
	};

	// Every link on the route stays on the route. The artifact build uses
	// SvelteKit's hash router, so a link to an id on the page would be read
	// as a route; this handler moves focus there itself.
	async function onClick(event: MouseEvent) {
		const anchor = (event.target as Element | null)?.closest('a');
		const href = anchor?.getAttribute('href');
		if (!href) return;
		const outside = /\/(terms|privacy)$/.exec(href);
		if (outside) {
			event.preventDefault();
			prototype.note = OUTSIDE[outside[1]];
			return;
		}
		if (!href.startsWith('#')) return;
		event.preventDefault();
		if (!href.startsWith('#/')) {
			const target = document.querySelector<HTMLElement>(`[id="${CSS.escape(href.slice(1))}"]`);
			target?.scrollIntoView();
			target?.focus();
			return;
		}
		const query = new URLSearchParams(href.split('?', 2)[1] ?? '');
		const screen = query.get('to') as Screen | null;
		if (!screen) {
			const name = query.get('nav') ?? '';
			prototype.note = OUTSIDE[name] ?? `"${anchor?.textContent?.trim()}" is not in this prototype.`;
			return;
		}
		// #1516 counts the presses from the empty Practice to First Value.
		if (screen === 'name' && prototype.screen === 'overview') prototype.presses = 1;
		await open(screen);
	}

	async function open(screen: Screen) {
		prototype.note = '';
		prototype.go(screen);
		await tick();
		window.scrollTo(0, 0);
		document.querySelector<HTMLElement>('#main')?.focus();
	}

	const signOut = async () => ({ ok: true }) as const;
	const isClientScreen = $derived(['name', 'start', 'engagement', 'client'].includes(prototype.screen));
	const navItems = $derived([
		{ label: 'Overview', href: to('overview'), current: prototype.screen === 'overview' },
		{ label: 'Clients', href: prototype.client ? to('client') : nowhere('Clients'), current: isClientScreen },
		...['Schedule', 'On call', 'Invoices', 'Contracts', 'Credits', 'Staff', 'Offers', 'Settings'].map((label) => ({
			label,
			href: nowhere(label),
			current: false
		}))
	]);
</script>

<svelte:document onclick={onClick} />

<Link href="#main" label="Skip to main content" variant="skip" />
{#if prototype.screen === 'signup' || !prototype.account}
	<SignedOutTopBar />
	<main id="main" tabindex="-1">
		{#key prototype.run}<SignupScreen />{/key}
	</main>
{:else}
	<StaffTopBar
		{navItems}
		practices={[
			{
				practiceId: 'p_1496',
				practiceName: prototype.account.practiceName,
				roles: ['owner', 'admin', 'doula'],
				href: to('overview')
			}
		]}
		currentPracticeId="p_1496"
		name={prototype.ownerName}
		email={prototype.account.email}
		accountHref={nowhere('Your account')}
		{signOut}
	/>
	{#key prototype.screen}
		<StaffFeedback
			email={prototype.account.email}
			practiceName={prototype.account.practiceName}
			roles={['owner', 'admin', 'doula']}
			onSend={async () => {}}
		/>
	{/key}
	<main id="main" tabindex="-1">
		{#key prototype.screen}
			{#if prototype.screen === 'overview'}
				<OverviewScreen />
			{:else if prototype.screen === 'name'}
				<NameScreen />
			{:else if prototype.screen === 'start'}
				<StartWorkScreen />
			{:else if prototype.screen === 'engagement'}
				<EngagementScreen />
			{:else}
				<ClientScreen />
			{/if}
		{/key}
	</main>
{/if}

<!-- Room for the prototype bar, so that it never hides the last control. -->
<div class="bar-room"></div>
<ControlBar onOpen={open} />

<style>
	.bar-room {
		block-size: var(--space-12);
	}
</style>
