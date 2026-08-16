<script lang="ts">
	import { resolve } from '$app/paths';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import { atomPages, moleculePages } from './components.js';

	const componentPages = [...atomPages, ...moleculePages];

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
				<a href={resolve('/style-guide')}>Overview</a>
				{#each componentPages as componentPage (componentPage.slug)}
					<a href={resolve(`/style-guide/${componentPage.slug}`)}>{componentPage.name}</a>
				{/each}
			</cluster-l>
		</nav>

		{@render children()}
	</stack-l>
</box-l>
