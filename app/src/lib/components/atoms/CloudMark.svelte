<script lang="ts">
	/*
	 * The Doula Cloud mark: three nested arcs, plum at the front and the
	 * decorative hairline at the back. Adopted from the teaser social cards
	 * rather than redesigned (#431) -- logo design is out of scope on the
	 * design map -- and recoloured into the token ramp.
	 *
	 * Only width and height vary by size. The canvas carries a `mark-stroke`
	 * ramp (9/4/3) because pen.dev's `strokeWidth` is node pixels rather
	 * than viewBox units and so does not scale with the frame; real SVG
	 * strokes do scale, so one `stroke-width` of 14 in a 202-unit viewBox
	 * renders as ~8.3/4.2/2.8 device pixels at the three sizes -- the ramp
	 * the drawing had to state by hand.
	 */
	interface Properties {
		size?: 'sm' | 'md' | 'lg';
		/*
		 * Decorative by default: the mark always sits beside the wordmark in
		 * BrandLockup, so naming it here would make a screen reader say
		 * "Doula Cloud" twice. A caller using the mark alone passes a label.
		 */
		label?: string;
	}

	let { size = 'sm', label }: Properties = $props();

	const dimensions = {
		sm: { width: 40, height: 19 },
		md: { width: 60, height: 28 },
		lg: { width: 120, height: 56 }
	} as const;

	const { width, height } = $derived(dimensions[size]);
</script>

<svg
	viewBox="48 76 202 94"
	{width}
	{height}
	fill="none"
	stroke-width="14"
	stroke-linecap="round"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
	focusable="false"
>
	<path class="back" d="M100 160a32 32 0 0 1 64 0" />
	<path class="middle" d="M78 160a54 54 0 0 1 104.61-18.82 22 22 0 0 1 33.39 18.82" />
	<path class="front" d="M56 160a76 76 0 0 1 137.97-44 44 44 0 0 1 44.03 44" />
</svg>

<style>
	@layer components {
		svg {
			display: inline-block;
			vertical-align: middle;
		}

		/* Drawn back to front, so the front arc is painted last and the
		   overlaps read as depth rather than as three crossing lines. */
		.back {
			stroke: var(--color-outline-variant);
		}

		.middle {
			stroke: var(--color-primary-hover);
		}

		.front {
			stroke: var(--color-primary);
		}
	}
</style>
