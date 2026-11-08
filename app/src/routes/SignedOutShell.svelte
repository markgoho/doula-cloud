<script lang="ts">
	import type { Snippet } from 'svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PortalFooter from '#lib/components/organisms/PortalFooter.svelte';
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';

	interface Properties {
		isPortalFooterShown?: boolean;
		children: Snippet;
	}

	let { isPortalFooterShown = false, children }: Properties = $props();
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<SignedOutTopBar />
<main id="main" tabindex="-1">
	{@render children()}
</main>
{#if isPortalFooterShown}
	<PortalFooter />
{/if}
