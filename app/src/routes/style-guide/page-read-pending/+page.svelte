<script lang="ts">
	import PageReadPending from '#lib/components/molecules/PageReadPending.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Page read, not landed" />
	<Text
		text="What a Template draws inside its frame while the page's read has not landed: the page title over the reason it failed, or over bars while it loads."
		tone="variant"
	/>

	<stack-l space="var(--space-section)" data-load-error>
		<PageReadPending
			title="Ana-Sofía Kowalczyk-Brennan's Contract"
			read={{ status: 'failed', message: 'Your Contract did not load. Refresh the page to try again.' }}
			skeleton="text"
		/>
	</stack-l>
	<stack-l space="var(--space-section)">
		<PageReadPending title="Clients" read={{ status: 'loading', label: 'Loading Clients' }} skeleton="row" />
	</stack-l>
</stack-l>
