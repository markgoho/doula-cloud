<script lang="ts">
	import { resolve } from '$app/paths';
	import { atomPages, moleculePages, organismPages } from './components.js';
	import Link from '#lib/components/atoms/Link.svelte';

	// Slugs are now filesystem-derived (non-literal), so resolve() can't
	// match them against its per-route overloads -- see +layout.svelte's
	// styleGuideRoot for the same workaround.
	const styleGuideRoot = resolve('/style-guide');
	const componentHref = (slug: string) => `${styleGuideRoot}/${slug}`;
</script>

<section>
	<h2>The whole continuum</h2>
	<ul>
		<li>
			<Link href={componentHref('drag-surface')} label="Drag surface" />
			-- any component in a frame you resize by dragging, so its behavior is watched rather than
			read about. The per-component entries below stay as they are.
		</li>
	</ul>

	<h2>Atoms</h2>
	<ul>
		{#each atomPages as atomPage (atomPage.slug)}
			<li><Link href={componentHref(atomPage.slug)} label={atomPage.name} /></li>
		{/each}
	</ul>

	<h2>Molecules</h2>
	<ul>
		{#each moleculePages as moleculePage (moleculePage.slug)}
			<li><Link href={componentHref(moleculePage.slug)} label={moleculePage.name} /></li>
		{/each}
	</ul>

	<h2>Organisms</h2>
	<ul>
		{#each organismPages as organismPage (organismPage.slug)}
			<li><Link href={componentHref(organismPage.slug)} label={organismPage.name} /></li>
		{/each}
	</ul>
</section>
