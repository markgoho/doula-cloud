<!--
The pre-launch teaser (#358): a signed letter beside a card. Copy word for
word from the copy revision on #362; the look is the brand sheet's
(docs/marketing/brand.md), not the #362 prototype's.

The letter comes first in the source, so a screen reader meets the note
before the form. The card sits beside it while there is room for both and
drops under it when there is not; nothing here measures the viewport
(ADR-0024).

No canonical tag: #368 set the teaser root's posture as indexable with
none, and docs/copy/support-page.md follows it.
-->
<script lang="ts">
	import SiteMark from '#lib/components/SiteMark.svelte';
	import WaitlistCard from '#lib/components/WaitlistCard.svelte';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';
	import { WAITLIST_FORM_ACTION } from '#lib/waitlist.js';

	const headline = "I'm building the thing you keep rebuilding in a spreadsheet.";
	const description = `${headline} ${PRODUCT_NAME} opens in January 2027.`;
</script>

<svelte:head>
	<title>{PRODUCT_NAME}: coming January 2027</title>
	<meta name="description" content={description} />
	<meta property="og:type" content="website" />
	<meta property="og:url" content="{SITE_ORIGIN}/" />
	<meta property="og:title" content={headline} />
	<meta property="og:description" content="{PRODUCT_NAME} opens in January 2027." />
	<meta property="og:image" content="{SITE_ORIGIN}/social-card.png" />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta
		property="og:image:alt"
		content="Doula. Scheduler. Bookkeeper. Contract writer. The last three are struck through: I'm building the other three."
	/>
	<meta name="twitter:card" content="summary_large_image" />
</svelte:head>

<main class="page">
	<div class="teaser">
		<article class="letter">
			<stack-l>
				<SiteMark />
				<p class="eyebrow">Coming January 2027</p>
				<h1>{headline}</h1>
				<p>
					If you do this work, you already know where the hours go. Chasing a signature. Remembering who
					is due when. Working out what you're owed, in a spreadsheet only you understand.
				</p>
				<p>
					{PRODUCT_NAME} is one place for all of it, whether it's just you or a dozen doulas. It opens in
					January 2027. It's not ready yet, and I'd rather tell you that than show you a demo.
				</p>
				<p>Put your name down and I'll write to you once, when it opens.</p>
				<p class="signature">Mark<br /><span>Building {PRODUCT_NAME}</span></p>
			</stack-l>
		</article>

		<div class="aside">
			<WaitlistCard action={WAITLIST_FORM_ACTION} />
		</div>
	</div>
</main>

<style>
	.page {
		padding-block: var(--space-10) var(--space-12);
		padding-inline: var(--page-gutter);
	}

	/* The sidebar pattern with the sidebar second in the source: the card
	   takes a basis of its own and the letter takes the rest, until the
	   letter would fall under half the row, and then the card wraps under
	   it. */
	.teaser {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-10);
		align-items: flex-start;
		max-inline-size: calc(var(--measure) + 24rem);
		margin-inline: auto;
	}

	.letter {
		flex-basis: 0;
		flex-grow: 999;
		min-inline-size: 55%;
		max-inline-size: var(--measure);
	}

	.aside {
		flex-basis: 20rem;
		flex-grow: 1;
	}

	.eyebrow {
		color: var(--color-primary);
		font-size: var(--text-label-size);
		font-weight: var(--text-label-weight);
		letter-spacing: var(--text-meta-tracking);
	}

	h1 {
		font-size: var(--text-display-size);
		font-weight: var(--text-display-weight);
		line-height: var(--text-display-leading);
		letter-spacing: var(--text-display-tracking);
	}

	.signature {
		font-weight: var(--text-label-weight);
	}

	.signature span {
		color: var(--color-on-surface-variant);
		font-weight: normal;
	}
</style>
