<script lang="ts">
	import { resolve } from '$app/paths';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
</script>

<PageTitle page="Demo" />
<Link href={resolve('/demo/playwright')} label="playwright" />
