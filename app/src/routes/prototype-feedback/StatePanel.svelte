<script module lang="ts">
	import type { Context, Kind } from './fixtures.js';

	export interface Report {
		view: 'host' | 'form' | 'sent';
		// What the address bar would read. Empty means "the screen's own URL".
		url: string;
		sent?: { kind: Kind; text: string; context: Context };
	}
</script>

<script lang="ts">
	/*
	 * PROTOTYPE -- #1502. Not part of any design: the state behind the
	 * variant, split the way #1501 Q1 splits it -- what Doula Cloud keeps
	 * under the BAA, and what reaches the private GitHub issue.
	 */
	import { copy, kinds, type HostScreen, type Shell } from './fixtures.js';

	interface Properties {
		report: Report;
		shell: Shell;
		screen: HostScreen;
		routeId: string;
	}

	let { report, shell, screen, routeId }: Properties = $props();

	const kindLabel = (kind: Kind) => kinds.find((each) => each.value === kind)?.label ?? kind;
</script>

<details class="panel">
	<summary>Prototype state</summary>
	<dl>
		<dt>View</dt>
		<dd>{report.view}</dd>
		<dt>Address bar</dt>
		<dd><code>{report.url || screen.path}</code></dd>
		<dt>Origin path</dt>
		<dd><code>{screen.path}</code></dd>
		<dt>Route pattern, from <code>match()</code></dt>
		<dd><code>{routeId}</code></dd>
	</dl>
	{#if report.sent}
		{@const sent = report.sent}
		<h2>Saved in Doula Cloud's database (BAA)</h2>
		<dl>
			<dt>Kind</dt>
			<dd>{kindLabel(sent.kind)}</dd>
			<dt>Free text</dt>
			<dd>{sent.text || '(empty)'}</dd>
			<dt>Full URL</dt>
			<dd><code>{sent.context.path}</code></dd>
			<dt>Sender</dt>
			<dd>{copy[shell].email}</dd>
			<dt>Practice</dt>
			<dd>{sent.context.practice}</dd>
			<dt>Role, build, width, browser, time</dt>
			<dd>
				{sent.context.role}, {sent.context.build}, {sent.context.width}px, {sent.context.browser},
				{sent.context.time}
			</dd>
		</dl>
		<h2>Reaches the private GitHub issue</h2>
		<dl>
			<dt>Title</dt>
			<dd>{kindLabel(sent.kind)} on <code>{sent.context.routeId}</code></dd>
			<dt>Label</dt>
			<dd><code>kind:{sent.kind}</code></dd>
			<dt>Body</dt>
			<dd>
				Route <code>{sent.context.routeId}</code>; role {sent.context.role}; build
				{sent.context.build}; {sent.context.width}px; {sent.context.browser}; {sent.context.time};
				read page <code>/founder/feedback/fb_01J9…</code>
			</dd>
		</dl>
	{/if}
</details>

<style>
	.panel {
		position: fixed;
		inset-block-end: var(--space-12);
		inset-inline-start: var(--space-2);
		z-index: 30;
		max-inline-size: min(28rem, calc(100vw - var(--space-4)));
		max-block-size: 70vh;
		overflow: auto;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius);
		background: var(--color-on-surface);
		color: var(--color-surface);
		font-size: var(--text-meta-size);
		box-shadow: 0 4px 16px rgb(0 0 0 / 30%);
	}

	summary {
		cursor: pointer;
	}

	h2 {
		margin: var(--space-3) 0 var(--space-1);
		font-size: var(--text-meta-size);
	}

	dt {
		opacity: 0.7;
	}

	dd {
		margin: 0 0 var(--space-1);
		overflow-wrap: anywhere;
	}
</style>
