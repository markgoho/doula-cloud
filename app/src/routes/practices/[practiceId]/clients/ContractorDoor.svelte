<script lang="ts">
	/*
	 * #501 (ADR-0017): "a contractor originates nothing" -- a contractor
	 * Doula's Add a Client is a door that only explains. Two routes show
	 * it in place of what the BFF would refuse her with a 403: the search
	 * (`SearchHandler`), and every intake question (`CreateHandler`),
	 * which #1609 made reachable without the search while a Practice
	 * holds no Client record. The refusal still stands at the BFF and in
	 * RLS; this door is only what she reads instead of a failure.
	 */
	import { resolve } from '$app/paths';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
</script>

<PageTitle page="Add a Client" />

<container-l>
	<center-l max="var(--form-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-6)">
			<Heading level={1} variant="page" text="Add a Client" />
			<Text
				tone="variant"
				measure
				text="Work at this Practice reaches you as an Offer, so there is no Client to search for or add here. To take on Clients of your own, set up a Practice."
			/>
			<cluster-l space="var(--space-3)" align="center">
				<Link href={resolve('/(signed-out)/signup')} label="Set up a Practice" />
			</cluster-l>
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}
	}
</style>
