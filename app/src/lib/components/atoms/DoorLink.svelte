<script lang="ts">
	/*
	 * A door: one link whose whole area is the target, with the door's name
	 * and one short line under it (#1645). The signed-out landing has three,
	 * one for each kind of reader who arrives there.
	 *
	 * Not a `Link` variant: a `Link` draws one label, and a door draws two
	 * pieces of text. So it is the second atom allowed a raw `<a>` (see
	 * `eslint.config.js`). When an `<a>` holds both pieces, the browser
	 * builds its accessible name from both, so a screen reader would
	 * announce "Staff sign in For doulas and practice owners." as the name.
	 * Here the name points at the name span alone, and the line is the
	 * link's description (`aria-describedby`), so the line is still heard,
	 * after the name.
	 *
	 * Two looks. `primary` is the filled door, with the arrow, for the one
	 * door most readers want; the accent is spent on it and nowhere else on
	 * the page. `secondary` is a bordered card whose name looks like every
	 * other link in the app -- underlined, in the link color -- because a
	 * thing that is a link should look like one (the brief's Law of
	 * Similarity).
	 *
	 * `menu` is a row in a menu button's panel (#2141): the Practice
	 * switcher's Practices, each one's name with its roles under it. The
	 * whole row is the target, as the drawing had it, where a `Link` with
	 * the roles beside it left the roles line dead to a press. Not
	 * underlined, for the reason `Link`'s own `menu` gives. `current`
	 * marks the Practice she is in: `aria-current`, a fill, and a tick, so
	 * color is not the only signal.
	 */
	import Icon from './Icon.svelte';

	interface Properties {
		href: string;
		label: string;
		description: string;
		variant?: 'primary' | 'secondary' | 'menu';
		current?: boolean;
	}

	let { href, label, description, variant = 'secondary', current = false }: Properties = $props();

	const id = $props.id();
	const nameId = `${id}-name`;
	const descriptionId = `${id}-description`;
</script>

<a {href} class={variant} aria-current={current ? 'page' : undefined} aria-labelledby={nameId} aria-describedby={descriptionId}><!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
	<span class="text">
		<span id={nameId} class="name">{label}</span>
		<span id={descriptionId} class="description">{description}</span>
	</span>
	{#if variant === 'primary'}
		<Icon name="arrow-right" size={24} weight="light" />
	{:else if current}
		<span class="tick"><Icon name="check" size={16} /></span>
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

		.text {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
			min-inline-size: 0;
		}

		.name {
			overflow-wrap: anywhere;
		}

		/* A line can quote a person's own words (#2007's last Message),
		   which may be one long link with nowhere to break. */
		.description {
			overflow-wrap: anywhere;
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

		.menu {
			gap: var(--space-3);
			min-block-size: var(--nav-row-height);
			padding: var(--space-2) var(--space-4);
			border: 0;
			border-radius: 0;
			color: var(--color-on-surface);
		}

		.menu:hover,
		.menu[aria-current='page'] {
			background-color: var(--color-surface-container);
		}

		.menu:focus-visible {
			outline-offset: calc(-1 * var(--focus-ring-width));
		}

		.menu .name {
			font-size: var(--text-body-sm-size);
		}

		.menu[aria-current='page'] .name {
			font-weight: var(--font-weight-semibold);
		}

		.menu .description {
			color: var(--color-on-surface-muted);
			font-size: var(--text-meta-size);
		}

		.tick {
			display: flex;
			color: var(--color-primary);
		}
	}
</style>
