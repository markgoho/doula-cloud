<script lang="ts">
	/*
	 * The page's `<h1>` over what stands in for a page whose read has not
	 * landed: the Notice that says it failed, or the Skeleton while it
	 * loads (#480). Templates draw it inside their own frame; it draws
	 * none.
	 */
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import type { PageReadState } from '#lib/pageRead.svelte.js';

	interface Properties {
		title: string;
		read: Exclude<PageReadState, { status: 'loaded' }>;
		/**
		`row` for a page of tables, `text` for one of prose and facts.
		*/
		skeleton: 'row' | 'text';
	}

	let { title, read, skeleton }: Properties = $props();
</script>

<Heading level={1} variant="page" text={title} />
{#if read.status === 'failed'}
	<Notice variant="error" message={read.message} />
{:else}
	<Skeleton variant={skeleton} lines={6} label={read.label} />
{/if}
