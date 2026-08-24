<script lang="ts">
	import { resolve } from '$app/paths';
	import { atomPages, moleculePages, organismPages } from './components.js';
	import Link from '#lib/components/atoms/Link.svelte';
</script>

<section>
	<h2>Atoms</h2>
	<ul>
		{#each atomPages as atomPage (atomPage.slug)}
			<li><Link href={resolve(`/style-guide/${atomPage.slug}`)} label={atomPage.name} /></li>
		{/each}
	</ul>

	<h2>Molecules</h2>
	<ul>
		{#each moleculePages as moleculePage (moleculePage.slug)}
			<li><Link href={resolve(`/style-guide/${moleculePage.slug}`)} label={moleculePage.name} /></li>
		{/each}
	</ul>

	<h2>Organisms</h2>
	<ul>
		{#each organismPages as organismPage (organismPage.slug)}
			<li><Link href={resolve(`/style-guide/${organismPage.slug}`)} label={organismPage.name} /></li>
		{/each}
	</ul>
</section>
