<script lang="ts">
	/**
	 * The one place that writes `<title>`. Not tiered under atoms/molecules/
	 * organisms/templates: it renders nothing visible, so it is not a design
	 * system component and earns no /style-guide page (`style-guide.spec.ts`
	 * only walks those four directories). Every Template calls this
	 * internally; every route with no Template calls it directly (#487).
	 */
	import { formatPageTitle } from '#lib/pageTitle.js';

	interface Properties {
		page: string;
		serviceName?: string;
		isError?: boolean;
	}

	let { page, serviceName, isError = false }: Properties = $props();

	const text = $derived(formatPageTitle(page, { serviceName, isError }));
</script>

<svelte:head>
	<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- formatPageTitle always returns a string -->
	<title>{text}</title>
	<!-- v8 ignore stop -->
</svelte:head>
