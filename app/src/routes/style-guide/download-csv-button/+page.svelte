<script lang="ts">
	import DownloadCsvButton from '#lib/components/molecules/DownloadCsvButton.svelte';

	const noop = async () => {};
</script>

<stack-l space="var(--space-6)">
	<h1>Download CSV button</h1>

	<section>
		<h2>Default</h2>
		<DownloadCsvButton isExporting={false} hasRows description="A spreadsheet of the 35 Contracts this list shows." onDownload={noop} />
	</section>

	<section>
		<h2>Flush</h2>
		<DownloadCsvButton isExporting={false} hasRows isFlush description="A spreadsheet of the 41 Clients this list shows." onDownload={noop} />
	</section>

	<section>
		<h2>Nothing to download</h2>
		<DownloadCsvButton isExporting={false} hasRows={false} description="A spreadsheet of every Contract this list shows." onDownload={noop} />
	</section>

	<section>
		<h2>Downloading</h2>
		<DownloadCsvButton isExporting hasRows description="A spreadsheet of the 35 Contracts this list shows." onDownload={noop} />
	</section>
</stack-l>
