<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import {
		atomPages,
		moleculePages,
		organismPages,
		templatePages,
		templateSlugs
	} from './components.js';

	const componentPages = [...atomPages, ...moleculePages, ...organismPages, ...templatePages];

	/*
	 * `resolve()` is overloaded per route, so handing it a union of every
	 * component slug stops resolving once the union is large enough -- and
	 * this list is now over thirty. So resolve the one static parent, which
	 * is what carries any configured base path, and append the slug.
	 */
	const styleGuideRoot = resolve('/style-guide');
	const componentHref = (slug: string) => `${styleGuideRoot}/${slug}`;

	let { children } = $props();

	/*
	 * A Template owns its own page gutters and max-width (ADR-0018), so
	 * rendering one inside this page's padded, bordered wrapper shows
	 * nothing like what the app shows. Template pages get the full window
	 * instead, inside a frame that stands in for the viewport edge.
	 */
	const isTemplatePage = $derived(
		templateSlugs.includes(page.url.pathname.split('/').findLast(Boolean) ?? '')
	);

	let isDark = $state(false);

	function onToggleTheme(isChecked: boolean) {
		isDark = isChecked;
		document.documentElement.dataset.theme = isChecked ? 'dark' : 'light';
	}
</script>

<box-l>
	<stack-l space="var(--space-6)">
		<cluster-l justify="space-between" align="center">
			<h1>Style guide</h1>
			<label>
				Dark mode
				<Checkbox variant="toggle" checked={isDark} onChange={onToggleTheme} />
			</label>
		</cluster-l>

		<nav>
			<cluster-l space="var(--space-4)">
				<Link href={resolve('/style-guide')} label="Overview" />
				{#each componentPages as componentPage (componentPage.slug)}
					<Link href={componentHref(componentPage.slug)} label={componentPage.name} />
				{/each}
			</cluster-l>
		</nav>

		{#if !isTemplatePage}
			{@render children()}
		{/if}
	</stack-l>
</box-l>

{#if isTemplatePage}
	<div class="viewport">
		{@render children()}
	</div>
{/if}

<style>
	@layer components {
		.viewport {
			margin: var(--space-6);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface);
			overflow: hidden;
		}
	}
</style>
