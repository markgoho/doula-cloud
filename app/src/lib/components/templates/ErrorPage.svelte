<script lang="ts">
	/*
	 * Archetype for the page nobody wrote (#471): a `+error.svelte` renders
	 * this instead of falling through to SvelteKit's default. GOV.UK's three
	 * patterns -- page not found, there is a problem with the service,
	 * service unavailable -- plus the refusal ADR-0008's read gate produces.
	 * Each `kind` answers the one thing GOV.UK says the person actually
	 * needs: whether trying again will help.
	 *
	 * No chrome here, per ADR-0018 -- the nearest `+layout.svelte` above the
	 * failure renders it. This owns only the gutter, the max-width, and the
	 * copy.
	 */
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { ErrorKind } from '#lib/errorPage.js';

	interface Properties {
		kind: ErrorKind;
		wayOutHref: string;
		wayOutLabel: string;
	}

	let { kind, wayOutHref, wayOutLabel }: Properties = $props();

	const copyByKind: Record<ErrorKind, { title: string; body: string }> = {
		notFound: {
			title: 'Page not found',
			body: 'Check the web address is correct, or the page may have been removed. Trying again will not change that.'
		},
		refused: {
			title: 'You cannot view this',
			body: 'Your role does not have permission to see this. Trying again will not change that.'
		},
		unavailable: {
			title: 'Doula Cloud is unavailable',
			body: 'This is planned, and does not mean anything went wrong. Try again shortly.'
		},
		problem: {
			title: 'Sorry, there is a problem',
			body: 'Something went wrong on our end, not because of anything you did. Try again in a few minutes.'
		}
	};

	const copy = $derived(copyByKind[kind]);
</script>

<PageTitle page={copy.title} />

<container-l>
	<center-l max="var(--page-max)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-4)">
			<Heading level={1} variant="page" text={copy.title} />
			<Text text={copy.body} />
			<Link href={wayOutHref} label={wayOutLabel} variant="primary" />
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
