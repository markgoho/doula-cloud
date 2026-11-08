<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isRefused?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Credential field, refused', props: { isRefused: true } }
	];
</script>

<script lang="ts">
	import CredentialField from '#lib/components/molecules/CredentialField.svelte';

	let { isRefused = false }: Properties = $props();

	let email = $state('maya.okonkwo-whitfield@birthcollective.example');
	let password = $state('');
</script>

<stack-l space="var(--space-4)">
	<h1>Credential field</h1>
	<CredentialField
		id="demo-email"
		label="Email address"
		type="email"
		autocomplete="username"
		value={email}
		onInput={(value) => (email = value)}
	/>
	<CredentialField
		id="demo-password"
		label="Password"
		type="password"
		autocomplete="current-password"
		value={password}
		onInput={(value) => (password = value)}
		error={isRefused ? 'Enter your password' : undefined}
	/>
</stack-l>
