<script lang="ts">
	// GOV.UK's summary list, as check-your-answers uses it: a key, its
	// answer, and a Change button where the answer can be changed.
	let {
		rows
	}: { rows: { key: string; value: string; onchange?: () => void }[] } = $props();
</script>

<dl>
	{#each rows as row (row.key)}
		<div class="row">
			<dt>{row.key}</dt>
			<dd>{row.value}</dd>
			<dd class="action">
				{#if row.onchange}
					<button type="button" class="change" onclick={row.onchange}>
						Change<span class="visually-hidden"> {row.key.toLowerCase()}</span>
					</button>
				{/if}
			</dd>
		</div>
	{/each}
</dl>

<style>
	dl {
		max-inline-size: var(--measure);
		margin: 0;
	}

	.row {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1) var(--space-4);
		padding-block: var(--space-3);
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	dt {
		flex: 1 1 12rem;
		font-weight: bold;
	}

	dd {
		flex: 2 1 14rem;
		margin: 0;
		overflow-wrap: anywhere;
	}

	.action {
		flex: 0 0 7ch;
	}

	.change {
		padding: 0;
		color: var(--color-primary);
		text-decoration: underline;
		background: none;
		border: none;
		cursor: pointer;
	}

	.change:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}

	.visually-hidden {
		position: absolute;
		inline-size: 1px;
		block-size: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
</style>
