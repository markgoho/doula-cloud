<script lang="ts">
	import DownloadCsvNotices from '#lib/components/molecules/DownloadCsvNotices.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Download CSV notices</h1>

	<section>
		<h2>Downloaded</h2>
		<DownloadCsvNotices refusal="" downloaded="Downloaded contracts-2026-11-14.csv, 17 Contracts." />
	</section>

	<section>
		<h2>Refused</h2>
		<DownloadCsvNotices refusal="Your Contracts did not download. Nothing you saved is lost." downloaded="" />
	</section>
</stack-l>
