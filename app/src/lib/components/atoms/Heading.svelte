<script lang="ts">
	/*
	 * Two axes, two owners (#417). `level` is document structure -- it sets
	 * the screen-reader outline, and only the page's author knows it.
	 * `variant` is what the heading IS, and the design system decides how
	 * big that looks. The map below is the single place a size lives.
	 *
	 * The brief's `display` step is deliberately unreachable here: it is
	 * allowed on "the one page title on a hub", so the OverviewHub Template
	 * (#422) owns it, and the signed-out landing's greeting -- a paragraph,
	 * not a heading -- is the one other use, owned by `LandingPage` (#1645).
	 * A rule that depends on everyone remembering it is not a rule.
	 */
	interface Properties {
		level: 1 | 2 | 3 | 4 | 5 | 6;
		variant?: 'page' | 'section' | 'card';
		text: string;
		/**
		Lets a caller point an `aria-labelledby` at this heading -- RecordDetail's section landmarks (#507). spelling:ignore: aria-labelledby is an ARIA attribute name
		*/
		id?: string;
		/**
		 * A heading that replaces what was on screen takes focus the moment
		 * it appears (#816, #2018): replacing a form under a keyboard or
		 * screen reader user without moving her focus leaves her tabbing
		 * forward through a page she was never told had changed. The
		 * heading takes tabindex="-1" so it can hold focus without ever
		 * joining the tab order, which is the GOV.UK pattern for exactly
		 * this. A boolean rather than an attachment passthrough keeps the
		 * atom closed: taking focus on arrival is the one thing a heading
		 * is ever asked to do beyond being read.
		 */
		focusOnAppearing?: boolean;
	}

	let { level, variant, text, id, focusOnAppearing = false }: Properties = $props();

	function takeFocus(element: Element) {
		(element as HTMLElement).focus();
	}

	const fallbackByLevel = {
		1: 'page',
		2: 'section',
		3: 'card',
		4: 'card',
		5: 'card',
		6: 'card'
	} as const;
	const resolved = $derived(variant ?? fallbackByLevel[level]);
	const tag = $derived(`h${level}`);
	const classes = $derived(`variant-${resolved}`);
</script>

<svelte:element
	this={tag}
	{id}
	class={classes}
	class:takes-focus={focusOnAppearing}
	tabindex={focusOnAppearing ? -1 : undefined}
	{@attach focusOnAppearing && takeFocus}>{text}</svelte:element>

<style>
	@layer components {
		.variant-page,
		.variant-section,
		.variant-card {
			margin: 0;
			font-family: var(--font-family-base);
			color: var(--color-on-surface);
			/*
			 * #595: a heading can interpolate a Practice's own name (the
			 * Practice landing hub's title did, until #2141), and a Practice's registered name is one this repo has already
			 * seen typed as a bare URL (#530) -- no space or hyphen for a
			 * browser to break on. Every other free-text surface this repo
			 * ships (`Link`, `DataTable`, `SummaryList`, `ContractView`)
			 * already carries this same rule; a heading had simply never
			 * been measured with hostile content before this ticket's sweep
			 * found it 600px past its 320px frame.
			 *
			 * `min-inline-size: 0` travels with it for the same reason
			 * `Link.svelte`'s own comment gives: `RecordDetail`/`OverviewHub`
			 * render the level-1 heading as a `cluster-l` flex item beside an
			 * optional actions region, and a flex item's automatic minimum
			 * size is its min-content size -- the URL's full width -- which
			 * `overflow-wrap` alone cannot shrink past. Harmless where a
			 * heading is not a flex item at all: this only changes a flex
			 * item's own floor.
			 */
			overflow-wrap: anywhere;
			min-inline-size: 0;
		}

		/*
		 * The heading's own focus style (#2143). Left to the browser, a
		 * heading that took focus from the keyboard drew a thin black box
		 * the width of the whole column, which read as a selected field.
		 * The ring is the one a Link draws, and it hugs the words: the
		 * heading is as wide as its text, never wider than the column. On
		 * `:focus-visible`, so the ring shows where the browser judges the
		 * reader is on the keyboard and not after a click.
		 */
		.takes-focus {
			inline-size: fit-content;
		}

		.takes-focus:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.variant-page {
			font-size: var(--text-heading-lg-size);
			font-weight: var(--text-heading-lg-weight);
			line-height: var(--text-heading-lg-leading);
			letter-spacing: var(--text-heading-lg-tracking);
		}

		.variant-section {
			font-size: var(--text-heading-size);
			font-weight: var(--text-heading-weight);
			line-height: var(--text-heading-leading);
			letter-spacing: var(--text-heading-tracking);
		}

		.variant-card {
			font-size: var(--text-subheading-size);
			font-weight: var(--text-subheading-weight);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
		}
	}
</style>
