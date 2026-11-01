<script lang="ts">
	/*
	 * #2270: what a contractor reads at the address of an Engagement once
	 * her Attachment on it has ended, in place of "Page not found". The
	 * sentence is the BFF's, with the day it ended; nothing else about the
	 * Engagement reaches her. The words are from three persona consults
	 * on #2270. ErrorPage's frame, since this is the page she meets in
	 * place of one.
	 */
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	let { message }: { message: string } = $props();

	const title = 'Your work has ended';
</script>

<PageTitle page={title} />

<container-l>
	<center-l max="var(--measure)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-4)">
			<Heading level={1} variant="page" text={title} />
			<Text text={message} />
			<Text text="Your fee, the terms and what you were paid stay on Your offers." />
			<Link
				href={resolve('/practices/[practiceId]/offers', { practiceId: page.params.practiceId! })}
				label="Your offers"
				variant="primary"
			/>
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
