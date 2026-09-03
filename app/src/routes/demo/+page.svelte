<script lang="ts">
	import { resolve } from '$app/paths';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
</script>

<PageTitle page="Demo" />
<!-- #595: every page needs one accessible level-1 heading -- this route
     had none, which also left it with nothing the continuum sweep's
     readyText could match. -->
<Heading level={1} text="Demo" />
<Link href={resolve('/demo/playwright')} label="playwright" />
