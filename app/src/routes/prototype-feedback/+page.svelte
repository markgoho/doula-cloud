<script lang="ts">
	/*
	 * PROTOTYPE -- throwaway, wayfinder ticket #1502. Three variants of the
	 * pilot banner and the feedback page, switchable via `?variant=`, in
	 * both shells via `?shell=staff|portal`, mounted under the real
	 * StaffTopBar and PortalTopBar with fixture props. The variants differ
	 * in where the form lives relative to the screen the person came from,
	 * which is what decides how the origin is known and how they get back.
	 *
	 * Run: `bun run dev` in app/, then open /prototype-feedback.
	 */
	import { replaceState } from '$app/navigation';
	import { match } from '$app/paths';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import PortalTopBar from '#lib/components/organisms/PortalTopBar.svelte';
	import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
	import StatePanel, { type Report } from './StatePanel.svelte';
	import VariantA from './VariantA.svelte';
	import VariantB from './VariantB.svelte';
	import VariantC from './VariantC.svelte';
	import VariantD from './VariantD.svelte';
	import { copy, PRACTICE_NAME, screens, type Shell } from './fixtures.js';

	const variants = [
		{ key: 'A', name: 'Own page, origin in ?from=', component: VariantA },
		{ key: 'B', name: 'Dialog over the screen', component: VariantB },
		{ key: 'C', name: 'Form at the foot of every screen', component: VariantC },
		{ key: 'D', name: 'A, then a Notice back on the origin', component: VariantD }
	] as const;

	const initial = new URLSearchParams(globalThis.location?.search ?? '');
	let variantKey = $state(initial.get('variant') ?? 'A');
	let shell = $state<Shell>(initial.get('shell') === 'portal' ? 'portal' : 'staff');
	let screenKey = $state(initial.get('screen') ?? '');

	const current = $derived(variants.find((each) => each.key === variantKey) ?? variants[0]);
	const screen = $derived(screens[shell].find((each) => each.key === screenKey) ?? screens[shell][0]);

	let routeId = $state('');
	$effect(() => {
		const path = screen.path;
		void match(path).then((route) => (routeId = route?.id ?? '(no route matched)'));
	});

	let report = $state<Report>({ view: 'host', url: '' });

	$effect(() => {
		const parameters = new URLSearchParams({ variant: current.key, shell, screen: screen.key });
		replaceState(`?${parameters}`, {});
	});

	function cycle(step: number) {
		const index = variants.findIndex((each) => each.key === variantKey);
		variantKey = variants[(index + step + variants.length) % variants.length].key;
	}

	function onKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;
		if (target?.isContentEditable) return;
		if (event.key === 'ArrowLeft') cycle(-1);
		if (event.key === 'ArrowRight') cycle(1);
	}

	const signOut = async () => ({ ok: true }) as const;
	const staffNav = ['Overview', 'Clients', 'Schedule', 'On call', 'Invoices', 'Settings'].map(
		(label) => ({ label, href: `#${label}`, current: label === 'Clients' })
	);
	const portalNav = [
		{ label: 'Your care', href: '#care', current: true },
		{ label: 'Messages', href: '#messages', current: false },
		{ label: 'Contract', href: '#contract', current: false }
	];
</script>

<svelte:window onkeydown={onKeydown} />
<PageTitle page="PROTOTYPE #1502 feedback" />

<Link href="#main" label="Skip to main content" variant="skip" />
{#if shell === 'staff'}
	<StaffTopBar
		navItems={staffNav}
		practices={[{ practiceId: 'p_7f3a', practiceName: PRACTICE_NAME, roles: ['owner'], href: '#' }]}
		currentPracticeId="p_7f3a"
		name="Jordan Blake"
		email={copy.staff.email}
		accountHref="#"
		{signOut}
	/>
{:else}
	<PortalTopBar
		practiceName={PRACTICE_NAME}
		switcherLabel={PRACTICE_NAME}
		navItems={portalNav}
		name="Alex Rivera"
		accountHref="#"
		{signOut}
	/>
{/if}

{#key `${current.key}-${shell}-${screen.key}`}
	<current.component
		{shell}
		{screen}
		{routeId}
		onReport={(next: Report) => (report = next)}
		onNavigate={(key: string) => (screenKey = key)}
	/>
{/key}

<StatePanel {report} {shell} {screen} {routeId} />

<div class="switcher">
	<button type="button" onclick={() => cycle(-1)} aria-label="Previous variant">←</button>
	<span class="label">{current.key} — {current.name}</span>
	<button type="button" onclick={() => cycle(1)} aria-label="Next variant">→</button>
	<label>
		<span>Shell</span>
		<select bind:value={shell} onchange={() => (screenKey = '')}>
			<option value="staff">Staff app</option>
			<option value="portal">Portal</option>
		</select>
	</label>
	<label>
		<span>Screen</span>
		<select value={screen.key} onchange={(event) => (screenKey = event.currentTarget.value)}>
			{#each screens[shell] as option (option.key)}
				<option value={option.key}>{option.title}</option>
			{/each}
		</select>
	</label>
</div>

<style>
	.switcher {
		position: fixed;
		inset-block-end: var(--space-3);
		inset-inline: var(--space-2);
		z-index: 20;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-1) var(--space-3);
		margin-inline: auto;
		inline-size: fit-content;
		max-inline-size: 100%;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius);
		background: var(--color-on-surface);
		color: var(--color-surface);
		font-size: var(--text-meta-size);
		box-shadow: 0 4px 16px rgb(0 0 0 / 30%);
	}

	.switcher button {
		border: 0;
		padding: var(--space-1) var(--space-2);
		background: none;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	label {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}
</style>
