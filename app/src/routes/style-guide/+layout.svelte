<script lang="ts">
	import { resolve } from '$app/paths';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import { atomPages } from './components.js';

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
				{#each atomPages as atomPage (atomPage.slug)}
					<a href={resolve(`/style-guide/${atomPage.slug}`)}>{atomPage.name}</a>
				{/each}
			</cluster-l>
		</nav>

		{@render children()}
	</stack-l>
</box-l>
