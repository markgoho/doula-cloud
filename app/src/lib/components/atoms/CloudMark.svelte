<script lang="ts">
	/*
	 * The mark: two strokes, the outer two-lobe line and the inner arch
	 * under its big lobe. docs/marketing/brand.md is the source of the path
	 * data and of each rule here, and the reference drawings are beside it
	 * in docs/marketing/brand/ (#1000, #1486). No third layer goes between
	 * the two strokes, and the geometry does not change.
	 *
	 * Only width and height vary by size. The canvas carries a `mark-stroke`
	 * ramp (9/4/3) because pen.dev's `strokeWidth` is node pixels rather
	 * than viewBox units and so does not scale with the frame; real SVG
	 * strokes do scale, so one `stroke-width` of 14 in a 202-unit viewBox
	 * renders as ~8.3/4.2/2.8 device pixels at the three sizes -- the ramp
	 * the drawing had to state by hand.
	 *
	 * `xl` is the signed-out landing's mark (#1645), standing alone on its
	 * own panel rather than beside a wordmark, so it is larger than any
	 * lockup step. It is the one size drawn into a space that may be
	 * narrower than it: its height follows its width, so a container that
	 * caps the mark scales it rather than squeezing it.
	 */
	interface Properties {
		size?: 'sm' | 'md' | 'lg' | 'xl';
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
		lg: { width: 120, height: 56 },
		// 200 wide keeps the viewBox's 202:94 proportion at 93 tall.
		xl: { width: 200, height: 93 }
	} as const;

	/*
	 * The brand sheet's threshold, as a height and not as a list of sizes:
	 * 28px tall or more, the inner arch takes the lighter plum. Below that
	 * the two tones are too near to each other to read as two, so the mark
	 * is one color.
	 */
	const TWO_TONE_MIN_HEIGHT = 28;

	const { width, height } = $derived(dimensions[size]);
	const tones = $derived(height >= TWO_TONE_MIN_HEIGHT ? 'two-tone' : 'one-color');
	const classes = $derived(`${tones} size-${size}`);
</script>

<svg
	viewBox="48 76 202 94"
	{width}
	{height}
	class={classes}
	fill="none"
	stroke-width="14"
	stroke-linecap="round"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
	focusable="false"
>
	<path class="arch" d="M100 160a32 32 0 0 1 64 0" />
	<path class="outline" d="M56 160a76 76 0 0 1 137.97-44 44 44 0 0 1 44.03 44" />
</svg>

<style>
	@layer components {
		svg {
			display: inline-block;
			vertical-align: middle;
		}

		/* reset.css caps every svg at `max-width: 100%` but leaves the height
		   attribute alone, so a capped mark would sit letterboxed in a 93px
		   box. `auto` takes the height from the viewBox's proportion instead. */
		.size-xl {
			block-size: auto;
		}

		/* The lockup sizes keep the width they declare (#1747). That same
		   percentage cap makes an svg "compressible" (CSS Sizing 3, 5.2.2):
		   its min-content contribution to the flex row holding it is zero,
		   so BrandLockup's own width was computed without the mark, and the
		   mark then drew in what was left -- 34.9px of its 40 in StaffTopBar,
		   at every width. With the cap gone the mark counts toward the
		   lockup's width. `flex: none` is the rule Icon carries (#1743): a
		   flex item's automatic minimum alone did not hold it, and the `lg`
		   mark still gave up 39px of its 120 beside a name too long for the
		   row. Only `xl` stands alone in a space that may be narrower than
		   it. */
		.size-sm,
		.size-md,
		.size-lg {
			flex: none;
			max-inline-size: none;
		}

		/* The inner arch is drawn first, so the outer line is painted last
		   and stays the strongest stroke in the two themes. */
		.arch,
		.outline {
			stroke: var(--color-primary);
		}

		.two-tone .arch {
			stroke: var(--color-primary-hover);
		}
	}
</style>
