<script lang="ts">
	/*
	 * A door: one link whose whole area is the target, with the door's name
	 * and one short line under it (#1645). The signed-out landing has three,
	 * one for each kind of reader who arrives there.
	 *
	 * Not a `Link` variant: a `Link` draws one label, and a door draws two
	 * pieces of text. So it is the second atom allowed a raw `<a>` (see
	 * `eslint.config.js`). Put both inside an `<a>` and the link's accessible name
	 * becomes both run together, so a screen reader would announce "Staff
	 * log in For doulas and practice owners." as the name. Here the name is
	 * the name alone (`aria-labelledby`, spelling:ignore: an ARIA attribute name) and the line is its description
	 * (`aria-describedby`), so the line is still heard, after the name.
	 *
	 * Two looks. `primary` is the filled door, with the arrow, for the one
	 * door most readers want; the accent is spent on it and nowhere else on
	 * the page. `secondary` is a bordered card whose name looks like every
	 * other link in the app -- underlined, in the link color -- because a
	 * thing that is a link should look like one (the brief's Law of
	 * Similarity).
	 */
	import Icon from './Icon.svelte';

	interface Properties {
		href: string;
		label: string;
		description: string;
		variant?: 'primary' | 'secondary';
	}

	let { href, label, description, variant = 'secondary' }: Properties = $props();

	const id = $props.id();
	const nameId = `${id}-name`;
	const descriptionId = `${id}-description`;
</script>

<a {href} class={variant} aria-labelledby={nameId} aria-describedby={descriptionId}><!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
	<span class="text">
		<span id={nameId} class="name">{label}</span>
		<span id={descriptionId} class="description">{description}</span>
	</span>
	{#if variant === 'primary'}
		<Icon name="arrow-right" size={24} weight="light" />
	{/if}
</a>

<style>
	@layer components {
		a {
			display: flex;
			align-items: center;
			justify-content: space-between;
			/* `Link`'s own `stack-l > a { align-self: start }` would shrink a
			   door to its text; a door is the whole row. */
			align-self: stretch;
			gap: var(--space-4);
			min-block-size: var(--hit-target-min);
			padding: var(--space-5);
			border: var(--border-thin) solid transparent;
			border-radius: var(--radius);
			font-family: var(--font-family-base);
			text-decoration: none;
			transition:
				background-color var(--motion-state) var(--ease-out),
				border-color var(--motion-state) var(--ease-out),
				color var(--motion-state) var(--ease-out);
		}

		a:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		a > :global(svg) {
			flex-shrink: 0;
		}

		.text {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
			min-inline-size: 0;
		}

		.name {
			overflow-wrap: anywhere;
		}

		.description {
			font-size: var(--text-body-size);
			font-weight: var(--text-body-weight);
			line-height: var(--text-body-leading);
			letter-spacing: var(--text-body-tracking);
		}

		.primary {
			background-color: var(--color-primary);
			color: var(--color-on-primary);
		}

		.primary:hover {
			background-color: var(--color-primary-hover);
		}

		.primary .name {
			font-size: var(--text-heading-size);
			font-weight: var(--text-heading-weight);
			line-height: var(--text-heading-leading);
			letter-spacing: var(--text-heading-tracking);
		}

		.secondary {
			flex-direction: column;
			align-items: flex-start;
			justify-content: flex-start;
			border-color: var(--color-outline-variant);
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface-variant);
		}

		.secondary:hover {
			border-color: var(--color-primary);
		}

		.secondary .name {
			color: var(--color-primary);
			font-size: var(--text-subheading-size);
			font-weight: var(--text-subheading-weight);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
			text-decoration: underline;
		}

		.secondary:hover .name {
			color: var(--color-primary-hover);
		}
	}
</style>
