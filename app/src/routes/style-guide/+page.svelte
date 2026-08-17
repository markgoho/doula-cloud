<script lang="ts">
	import { resolve } from '$app/paths';
	import { atomPages, moleculePages, organismPages } from './components.js';
</script>

<section>
	<h2>Atoms</h2>
	<ul>
		{#each atomPages as atomPage (atomPage.slug)}
			<li><a href={resolve(`/style-guide/${atomPage.slug}`)}>{atomPage.name}</a></li>
		{/each}
	</ul>

	<h2>Molecules</h2>
	<ul>
		{#each moleculePages as moleculePage (moleculePage.slug)}
			<li><a href={resolve(`/style-guide/${moleculePage.slug}`)}>{moleculePage.name}</a></li>
		{/each}
	</ul>

	<h2>Organisms</h2>
	<ul>
		{#each organismPages as organismPage (organismPage.slug)}
			<li><a href={resolve(`/style-guide/${organismPage.slug}`)}>{organismPage.name}</a></li>
		{/each}
	</ul>
</section>
