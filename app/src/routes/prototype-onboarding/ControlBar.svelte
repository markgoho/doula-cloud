<script lang="ts">
	/*
	 * PROTOTYPE -- #1496. Not part of any design: the reviewer's controls
	 * and the state behind the route. One line when closed, so that it does
	 * not hide the screen at 320px.
	 */
	import { CREDITS_PLACES, SCREENS, prototype, type Screen } from './model.svelte.js';

	interface Properties {
		onOpen: (screen: Screen) => void;
	}

	let { onOpen }: Properties = $props();

	let isOpen = $state(false);
	// The one variant is on the empty Practice, so the arrows show only there.
	const hasVariants = $derived(prototype.screen === 'overview' && !prototype.client);

	const place = $derived(CREDITS_PLACES.find((each) => each.key === prototype.creditsPlace) ?? CREDITS_PLACES[0]);
	const screenLabel = $derived(SCREENS.find((each) => each.key === prototype.screen)?.label ?? '');

	function cycle(step: number) {
		const index = CREDITS_PLACES.findIndex((each) => each.key === prototype.creditsPlace);
		prototype.creditsPlace = CREDITS_PLACES[(index + step + CREDITS_PLACES.length) % CREDITS_PLACES.length].key;
	}

	function onKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;
		if (!hasVariants || target?.isContentEditable) return;
		if (event.key === 'ArrowLeft') cycle(-1);
		else if (event.key === 'ArrowRight') cycle(1);
	}

	function startAgain() {
		prototype.startAgain();
		onOpen('signup');
	}
</script>

<svelte:window onkeydown={onKeydown} />

<!-- eslint-disable svelte/no-restricted-html-elements -- PROTOTYPE: the bar must look unlike the design it controls, so it uses no atom -->

<aside class="bar" aria-label="Prototype controls">
	{#if prototype.note}
		<p class="note" role="status">{prototype.note}</p>
	{/if}
	<div class="row">
		{#if hasVariants}
			<button type="button" onclick={() => cycle(-1)} aria-label="Previous variant">←</button>
			<span class="label">Credits sentence {place.key}</span>
			<button type="button" onclick={() => cycle(1)} aria-label="Next variant">→</button>
		{/if}
		<button
			type="button"
			class="text"
			aria-expanded={isOpen}
			aria-controls="prototype-panel"
			onclick={() => (isOpen = !isOpen)}
		>
			Prototype: {screenLabel}
		</button>
	</div>
	<div id="prototype-panel" hidden={!isOpen}>
		<div class="row">
			<button type="button" class="text" onclick={() => (prototype.fill += 1)}>Fill the form</button>
			<button type="button" class="text" onclick={startAgain}>Start again</button>
		</div>
		<div class="controls">
			<label>
				<span>Credits sentence on the empty Practice (screen 2)</span>
				<select bind:value={prototype.creditsPlace}>
					{#each CREDITS_PLACES as option (option.key)}
						<option value={option.key}>{option.key}: {option.name}</option>
					{/each}
				</select>
			</label>
			<label>
				<span>The Owner</span>
				<select bind:value={prototype.owner}>
					<option value="solo">Solo Owner: the Owner is the only Doula</option>
					<option value="agency">Agency Owner: 14 employee Doulas</option>
				</select>
			</label>
			<label>
				<span>Sample content for "Fill the form"</span>
				<select bind:value={prototype.content}>
					<option value="long">Long names (the 320px check)</option>
					<option value="typical">Short names, empty Practice name</option>
				</select>
			</label>
			<label>
				<span>Go to a screen</span>
				<select value={prototype.screen} onchange={(event) => onOpen(event.currentTarget.value as Screen)}>
					{#each SCREENS as option (option.key)}
						<option value={option.key}>{option.label}</option>
					{/each}
				</select>
			</label>
		</div>
		<dl>
			<dt>Practice</dt>
			<dd>{prototype.account?.practiceName ?? '(none yet)'}</dd>
			<dt>Owner</dt>
			<dd>{prototype.ownerName || '(none yet)'}</dd>
			<dt>Credits</dt>
			<dd>{prototype.balance}</dd>
			<dt>Client</dt>
			<dd>{prototype.clientName || '(none yet)'}</dd>
			<dt>Engagement</dt>
			<dd>
				{#if prototype.engagement}
					{prototype.engagement.kind}, Doula: {prototype.doulaLabel(prototype.engagement.doula)}
				{:else}
					(none yet)
				{/if}
			</dd>
			<dt>Presses since the empty Practice (#1516 says 3)</dt>
			<dd>{prototype.presses}</dd>
		</dl>
		{#if prototype.log.length > 0}
			<h2>Records written (the audit trail)</h2>
			<ol>
				{#each prototype.log as entry, index (index)}
					<li><code>{entry.record}</code> {entry.detail}</li>
				{/each}
			</ol>
		{/if}
	</div>
</aside>

<style>
	.bar {
		position: fixed;
		inset-block-end: var(--space-2);
		inset-inline: var(--space-2);
		z-index: 40;
		margin-inline: auto;
		inline-size: fit-content;
		max-inline-size: calc(100% - 2 * var(--space-2));
		max-block-size: 80dvb;
		overflow: auto;
		padding: var(--space-1) var(--space-3);
		border-radius: var(--radius);
		background: var(--color-on-surface);
		color: var(--color-surface);
		font-family: var(--font-family-base);
		font-size: var(--text-meta-size);
		line-height: var(--text-meta-leading);
		border: var(--border-thin) solid var(--color-outline);
	}

	.row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: 0 var(--space-2);
	}

	button {
		border: 0;
		padding: var(--space-1) var(--space-2);
		background: none;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	button.text {
		text-decoration: underline;
	}

	button:focus-visible,
	select:focus-visible {
		outline: var(--focus-ring-width) solid currentcolor;
		outline-offset: var(--focus-ring-offset);
	}

	.controls {
		display: grid;
		gap: var(--space-2);
		margin-block: var(--space-2);
	}

	label {
		display: grid;
		gap: var(--space-1);
	}

	select {
		max-inline-size: 100%;
		font: inherit;
	}

	.note {
		margin: var(--space-1) 0;
		max-inline-size: 40ch;
	}

	h2 {
		margin: var(--space-3) 0 var(--space-1);
		font-size: inherit;
	}

	dl,
	ol {
		margin: 0;
		padding: 0;
		max-inline-size: 48ch;
	}

	ol {
		padding-inline-start: var(--space-4);
	}

	dt {
		opacity: 0.75;
	}

	dd {
		margin: 0 0 var(--space-1);
		overflow-wrap: anywhere;
	}
</style>
