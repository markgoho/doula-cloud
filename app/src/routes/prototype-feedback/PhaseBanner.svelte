<script lang="ts">
	/*
	 * PROTOTYPE -- #1502. GOV.UK's phase banner shape: a tag, one sentence,
	 * one link. Same in every variant; only where the link goes changes.
	 */
	interface Properties {
		text: string;
		linkText: string;
		href: string;
		onActivate?: (event: MouseEvent) => void;
	}

	let { text, linkText, href, onActivate }: Properties = $props();
</script>

<div class="phase-banner">
	<p>
		<strong class="tag">Pilot</strong>
		<span>{text} <a {href} onclick={onActivate}>{linkText}</a>.</span>
	</p>
</div>

<style>
	.phase-banner {
		padding-block: var(--space-2);
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
		background: var(--color-surface);
	}

	p {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: var(--space-1) var(--space-3);
		margin: 0 auto;
		max-inline-size: 72rem;
		padding-inline: var(--space-4);
		font-size: var(--text-body-sm-size);
		line-height: var(--text-body-sm-leading);
		color: var(--color-on-surface);
	}

	span {
		flex: 1 1 16rem;
	}

	.tag {
		padding: 0 var(--space-2);
		border-radius: var(--radius);
		background: var(--color-info);
		color: var(--color-on-primary);
		font-size: var(--text-meta-size);
		font-weight: var(--font-weight-semibold);
		letter-spacing: var(--text-meta-tracking);
		text-transform: uppercase;
	}

	a {
		color: var(--color-primary);
	}
</style>
