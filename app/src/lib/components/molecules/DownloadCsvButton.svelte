<script lang="ts">
	/*
	 * The title row's "Download CSV" for a list (#2871, #2954). It is
	 * `ariaDisabled`, not disabled, while there is nothing to download, so it
	 * keeps its place and its Tab stop; `description` says how many rows the
	 * file will hold and is read with the button.
	 */
	import Button from '../atoms/Button.svelte';

	interface Properties {
		isExporting: boolean;
		hasRows: boolean;
		description: string;
		onDownload: () => Promise<void>;
		isFlush?: boolean;
	}

	let { isExporting, hasRows, description, onDownload, isFlush = false }: Properties = $props();

	const noteId = $props.id();
</script>

<div class="download" class:flush={isFlush}>
	<Button label="Download CSV" variant="secondary" loading={isExporting} ariaDisabled={!hasRows} describedBy={noteId} onClick={onDownload} />
	<span class="visually-hidden" id={noteId}>{description}</span>
</div>

<style>
	@layer components {
		/* The title row holds a start button's height, a little under a button's
		   target, so the button takes the difference out of its own margin and the
		   row, and the list under it, stay where they were. */
		.download {
			margin-block: calc((var(--start-button-block-size) - var(--hit-target-min)) / 2);
		}

		.flush {
			margin-block: 0;
		}
	}
</style>
