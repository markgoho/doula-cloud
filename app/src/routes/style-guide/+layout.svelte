<script lang="ts">
	import { resolve } from '$app/paths';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import { atomPages, moleculePages, organismPages } from './components.js';

	const componentPages = [...atomPages, ...moleculePages, ...organismPages];

	let { children } = $props();

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
					<Link href={resolve(`/style-guide/${componentPage.slug}`)} label={componentPage.name} />
				{/each}
			</cluster-l>
		</nav>

		{@render children()}
	</stack-l>
</box-l>
