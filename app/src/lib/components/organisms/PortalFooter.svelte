<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
	import { privacyPolicyUrl } from '#lib/legal.js';

	/*
	 * The foot of every Portal screen (#1558): the link to the Privacy
	 * Policy, which has a section written to the Client. Both Portal
	 * layouts, signed-out and authenticated, render this one component
	 * after `<main>`, so the link is in the same place on each screen,
	 * the first sign-in included.
	 *
	 * A notice and nothing more (ADR-0053): no sentence of agreement, no
	 * checkbox, nothing written when she follows it.
	 *
	 * `secondary`, not `primary`: the accent is spent on each screen's own
	 * action, and a footer link in the accent would compete with it. `Link`
	 * itself draws "(opens in new tab)" for an external href (#1557).
	 *
	 * It takes no props, so no Practice content can reach it.
	 */
</script>

<footer>
	<Link href={privacyPolicyUrl} label="Privacy Policy" variant="secondary" />
</footer>

<style>
	@layer components {
		footer {
			padding-block: var(--space-4);
			padding-inline: var(--page-gutter);
			border-block-start: var(--border-thin) solid var(--color-outline-variant);
			font-size: var(--text-body-sm-size);
		}

		/* Off the printed sheet (#2212), as the top bars are: a printed
		   Birth Plan goes to the hospital, and this footer is the app's
		   chrome, not the plan. */
		@media print {
			footer {
				display: none;
			}
		}

		/* The brief's Density rule (no hit target under 44px) is the `Link`
		   atom's own since #1828: the link is alone on its row, not inside
		   a paragraph, so the atom gives it the full 44px. */
	}
</style>
