<script lang="ts">
	import { resolve } from '$app/paths';
	import { atomPages } from './components.js';
</script>

<section>
	<h2>Atoms</h2>
	<ul>
		{#each atomPages as atomPage (atomPage.slug)}
			<li><a href={resolve(`/style-guide/${atomPage.slug}`)}>{atomPage.name}</a></li>
		{/each}
	</ul>

	<h2>Molecules</h2>
	<p>None yet.</p>

	<h2>Organisms</h2>
	<p>None yet.</p>
</section>
