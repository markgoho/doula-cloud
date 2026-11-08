<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Properties {
		control: 'box' | 'radio';
		children: Snippet;
	}

	let { control, children }: Properties = $props();
</script>

<section>
	<h2>Disabled</h2>
	<p>No caller disables a {control}. A disabled fieldset around a form still reaches one, and this is how it looks.</p>
	<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- the demo IS a disabled fieldset around the group, the one way a caller's form reaches a disabled control -->
	<fieldset disabled class="plain-fieldset">
		{@render children()}
	</fieldset>
</section>

<section>
	<h2>Hover and focus</h2>
	<p>Point at an option to see its halo. Tab to the group to see its focus outline.</p>
</section>

<style>
	@layer components {
		.plain-fieldset {
			margin: 0;
			padding: 0;
			border: none;
		}
	}
</style>
