<script lang="ts">
	import { signInWithEmailAndPassword } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { decideLanding, type Membership, type SessionInfo } from '#lib/landing.js';

	// PROTOTYPE — wayfinder ticket #94 visual-identity exploration. Not production code.
	const variants = ['A', 'B', 'C'] as const;
	const variantNames: Record<(typeof variants)[number], string> = {
		A: 'Warm Sage — soft card',
		B: 'Plum Dusk — split panel',
		C: 'Clay Air — minimal'
	};
	const variant = $derived(
		(variants.includes(page.url.searchParams.get('variant') as never)
			? page.url.searchParams.get('variant')
			: 'A') as (typeof variants)[number]
	);

	function setVariant(next: (typeof variants)[number]) {
		const url = new URL(page.url.href);
		url.searchParams.set('variant', next);
		goto(url, { replace: true, reset: false });
	}

	function cycle(direction: 1 | -1) {
		const i = variants.indexOf(variant);
		setVariant(variants[(i + direction + variants.length) % variants.length]);
	}

	let theme = $state<'light' | 'dark'>('light');

	function onKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable))
			return;
		if (event.key === 'ArrowLeft') cycle(-1);
		if (event.key === 'ArrowRight') cycle(1);
	}

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);
	let picker = $state<Membership[] | null>(null);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		picker = null;
		submitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			const response = await apiFetch('/api/staff/session', idToken);
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const session: SessionInfo = await response.json();
			const landing = decideLanding(session);
			if (landing.type === 'redirect') {
				await goto(resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
			} else {
				picker = landing.memberships;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#snippet pickerBlock()}
	{#if picker}
		<h2>Choose a Practice</h2>
		{#if picker.length === 0}
			<p>You don't belong to any Practice yet. Ask an Owner to invite you.</p>
		{:else}
			<ul class="picker-list">
				{#each picker as membership (membership.practiceId)}
					<li>
						<a href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}>
							{membership.practiceName}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
{/snippet}

{#snippet loginForm()}
	<form onsubmit={handleSubmit}>
		<label>
			<span>Email</span>
			<input type="email" bind:value={email} required />
		</label>
		<label>
			<span>Password</span>
			<input type="password" bind:value={password} required />
		</label>
		<button type="submit" disabled={submitting}>{submitting ? 'Logging in…' : 'Log in'}</button>
		{#if error}
			<p role="alert">{error}</p>
		{/if}
	</form>
	{@render pickerBlock()}
{/snippet}

{#if variant === 'A'}
	<main class="variant-a">
		<div class="card">
			<h1>Doula Cloud</h1>
			<p class="tagline">Welcome back. Log in to continue caring for your clients.</p>
			{@render loginForm()}
		</div>
	</main>
{:else if variant === 'B'}
	<main class="variant-b" data-theme={theme}>
		<aside class="panel">
			<h1>Doula Cloud</h1>
			<p class="tagline">Every family, every visit, held in one calm place.</p>
		</aside>
		<div class="form-side">
			<h2>Log in</h2>
			{@render loginForm()}
		</div>
	</main>
{:else}
	<main class="variant-c">
		<h1>Doula Cloud</h1>
		<p class="tagline">Log in</p>
		{@render loginForm()}
	</main>
{/if}

{#if import.meta.env.DEV}
	<div class="prototype-switcher" role="toolbar" aria-label="Prototype variant switcher">
		<button type="button" onclick={() => cycle(-1)} aria-label="Previous variant">←</button>
		<span>{variant} — {variantNames[variant]}</span>
		<button type="button" onclick={() => cycle(1)} aria-label="Next variant">→</button>
		{#if variant === 'B'}
			<button type="button" onclick={() => (theme = theme === 'light' ? 'dark' : 'light')}>
				{theme === 'light' ? '🌙 dark' : '☀️ light'}
			</button>
		{/if}
	</div>
{/if}

<style>
	/* PROTOTYPE tokens — throwaway OKLCH candidates for wayfinder ticket #94. */

	main {
		min-height: 100vh;
		box-sizing: border-box;
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	form {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.picker-list {
		list-style: none;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	/* ---- Variant A: "Warm Sage" — soft centered card ---- */
	.variant-a {
		--bg: oklch(97% 0.012 90);
		--surface: oklch(99% 0.006 90);
		--text: oklch(27% 0.02 90);
		--muted: oklch(48% 0.02 90);
		--border: oklch(89% 0.02 90);
		--accent: oklch(55% 0.09 175);
		--accent-strong: oklch(44% 0.09 175);
		--accent-contrast: oklch(99% 0.005 175);
		--radius: 16px;
		--space: 1.5rem;
		font-family: system-ui, sans-serif;
		background: var(--bg);
		color: var(--text);
		display: grid;
		place-items: center;
		padding: 2rem;
	}
	.variant-a .card {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: 0 8px 24px oklch(30% 0.02 90 / 8%);
		padding: calc(var(--space) * 1.5);
		width: min(380px, 100%);
	}
	.variant-a h1 {
		font-family: ui-serif, Georgia, serif;
		font-size: 1.75rem;
		margin: 0 0 0.5rem;
	}
	.variant-a .tagline {
		color: var(--muted);
		margin: 0 0 var(--space);
		font-size: 0.95rem;
	}
	.variant-a input {
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 0.6rem 0.75rem;
		font-size: 1rem;
	}
	.variant-a button[type='submit'] {
		background: var(--accent);
		color: var(--accent-contrast);
		border: none;
		border-radius: 10px;
		padding: 0.7rem 1rem;
		font-weight: 600;
		cursor: pointer;
	}
	.variant-a button[type='submit']:hover {
		background: var(--accent-strong);
	}
	.variant-a a {
		color: var(--accent-strong);
	}

	/* ---- Variant B: "Plum Dusk" — split panel ---- */
	.variant-b {
		--bg: oklch(96% 0.015 320);
		--panel-bg: oklch(38% 0.08 320);
		--panel-text: oklch(96% 0.01 320);
		--text: oklch(24% 0.02 320);
		--muted: oklch(46% 0.02 320);
		--border: oklch(85% 0.02 320);
		--accent: oklch(48% 0.13 325);
		--accent-strong: oklch(40% 0.13 325);
		--accent-contrast: oklch(99% 0.005 325);
		--radius: 8px;
		font-family: system-ui, sans-serif;
		background: var(--bg);
		color: var(--text);
		display: grid;
		grid-template-columns: 1fr 1fr;
		min-height: 100vh;
	}
	.variant-b .panel {
		background: var(--panel-bg);
		color: var(--panel-text);
		display: flex;
		flex-direction: column;
		justify-content: center;
		padding: 3rem;
	}
	.variant-b .panel h1 {
		font-size: 2rem;
		margin: 0 0 0.75rem;
	}
	.variant-b .panel .tagline {
		font-size: 1.05rem;
		opacity: 0.85;
		max-width: 28ch;
	}
	.variant-b .form-side {
		display: flex;
		flex-direction: column;
		justify-content: center;
		padding: 3rem;
		max-width: 420px;
	}
	.variant-b input {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.6rem 0.75rem;
		font-size: 1rem;
	}
	.variant-b button[type='submit'] {
		background: var(--accent);
		color: var(--accent-contrast);
		border: none;
		border-radius: var(--radius);
		padding: 0.7rem 1rem;
		font-weight: 600;
		cursor: pointer;
	}
	.variant-b button[type='submit']:hover {
		background: var(--accent-strong);
	}
	.variant-b a {
		color: var(--accent-strong);
	}
	@media (max-width: 640px) {
		.variant-b {
			grid-template-columns: 1fr;
		}
	}
	.variant-b[data-theme='dark'] {
		--bg: oklch(21% 0.015 320);
		--panel-bg: oklch(16% 0.05 320);
		--panel-text: oklch(94% 0.01 320);
		--text: oklch(92% 0.01 320);
		--muted: oklch(65% 0.02 320);
		--border: oklch(32% 0.02 320);
		--accent: oklch(72% 0.14 325);
		--accent-strong: oklch(80% 0.14 325);
		--accent-contrast: oklch(18% 0.02 325);
	}
	.variant-b[data-theme='dark'] input {
		background: oklch(25% 0.02 320);
		color: var(--text);
	}

	/* ---- Variant C: "Clay Air" — minimal, no card, airy ---- */
	.variant-c {
		--bg: oklch(98% 0.008 60);
		--text: oklch(28% 0.02 60);
		--muted: oklch(50% 0.02 60);
		--border: oklch(87% 0.015 60);
		--accent: oklch(58% 0.13 45);
		--accent-strong: oklch(46% 0.13 45);
		--accent-contrast: oklch(99% 0.005 45);
		font-family:
			'Helvetica Neue', system-ui, sans-serif;
		background: var(--bg);
		color: var(--text);
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: 5rem 2rem;
		gap: 2rem;
	}
	.variant-c h1 {
		font-size: 1.1rem;
		font-weight: 500;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		margin: 0;
	}
	.variant-c .tagline {
		color: var(--muted);
		margin: -1.5rem 0 0;
		font-size: 0.9rem;
	}
	.variant-c form {
		width: min(320px, 100%);
	}
	.variant-c input {
		border: none;
		border-bottom: 1px solid var(--border);
		border-radius: 0;
		padding: 0.6rem 0.1rem;
		font-size: 1rem;
		background: transparent;
	}
	.variant-c input:focus {
		outline: none;
		border-bottom: 2px solid var(--accent);
	}
	.variant-c button[type='submit'] {
		background: transparent;
		color: var(--accent-strong);
		border: 1px solid var(--accent);
		border-radius: 999px;
		padding: 0.65rem 1.5rem;
		font-weight: 500;
		letter-spacing: 0.04em;
		cursor: pointer;
		margin-top: 0.5rem;
	}
	.variant-c button[type='submit']:hover {
		background: var(--accent);
		color: var(--accent-contrast);
	}
	.variant-c a {
		color: var(--accent-strong);
	}

	/* ---- Prototype switcher chrome (not part of the design being evaluated) ---- */
	.prototype-switcher {
		position: fixed;
		bottom: 1.5rem;
		left: 50%;
		transform: translateX(-50%);
		display: flex;
		align-items: center;
		gap: 0.75rem;
		background: #111;
		color: #fff;
		padding: 0.5rem 1rem;
		border-radius: 999px;
		box-shadow: 0 4px 16px rgb(0 0 0 / 30%);
		font-family: system-ui, sans-serif;
		font-size: 0.85rem;
		z-index: 999;
	}
	.prototype-switcher button {
		background: rgb(255 255 255 / 15%);
		color: #fff;
		border: none;
		border-radius: 999px;
		width: 1.75rem;
		height: 1.75rem;
		cursor: pointer;
	}
	.prototype-switcher button:hover {
		background: rgb(255 255 255 / 30%);
	}
</style>
