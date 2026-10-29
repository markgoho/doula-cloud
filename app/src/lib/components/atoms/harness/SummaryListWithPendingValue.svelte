<!--
	Test support, not a component anybody ships.

	A summary list row whose figure has not landed yet (#2255): its label,
	and a one-line `Skeleton` in the value's place, as a screen draws it
	while its read runs. A spec cannot hand `SummaryList` a snippet without
	a component around both.
-->
<script lang="ts">
	import SummaryList from '../../molecules/SummaryList.svelte';
	import Skeleton from '../Skeleton.svelte';

	let { label }: { label: string } = $props();
</script>

{#snippet pending()}
	<Skeleton variant="text" lines={1} />
{/snippet}

<SummaryList rows={[{ label, value: '' }]} value={pending} />
