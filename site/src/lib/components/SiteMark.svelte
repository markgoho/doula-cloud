<!--
@component
The mark, as docs/marketing/brand.md draws it: two strokes, the outer line
in `--color-primary` and the inner arch in `--color-primary-hover`. It is
40px tall, above the 28px where the sheet asks for two tones, and 86px
wide, the viewBox's own ratio.

The app's CloudMark still draws the three-arc mark until #1486 lands.
When it does, this component can go and the teaser can use CloudMark.
Decorative by default, since the name is beside it.
-->
<svg
	viewBox="48 76 202 94"
	width="86"
	height="40"
	fill="none"
	stroke-width="14"
	stroke-linecap="round"
	aria-hidden="true"
	focusable="false"
>
	<path class="baby" d="M100 160a32 32 0 0 1 64 0" />
	<path class="mother" d="M56 160a76 76 0 0 1 137.97-44 44 44 0 0 1 44.03 44" />
</svg>

<style>
	svg {
		display: block;
	}

	.baby {
		stroke: var(--color-primary-hover);
	}

	.mother {
		stroke: var(--color-primary);
	}
</style>
