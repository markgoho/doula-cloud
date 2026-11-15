import { onMount } from 'svelte';
import type { CsvExportResult } from './csvExport.js';
import { saveFile } from './csvExport.js';

/**
 * What a list's "Download CSV" holds while it runs (#2871, #2954): whether
 * script is there to hand the file over, the press in flight, a refusal said
 * in place, and the words after a finished download, which belong to the
 * list as it was read and are dropped once it changes. Money is not offline
 * (ADR-0059): a refusal leaves the button for another try.
 */
export class CsvDownload<Key> {
	#downloaded = $state.raw<{ key: Key; words: string }>();
	isScripted = $state(false);
	isExporting = $state(false);
	refusal = $state('');

	constructor() {
		onMount(() => {
			this.isScripted = true;
		});
	}

	/**
	 * The words of the last download while `isCurrent` says the list still is
	 * the one it came from.
	 */
	wordsWhile(isCurrent: (key: Key) => boolean): string {
		return this.#downloaded && isCurrent(this.#downloaded.key) ? this.#downloaded.words : '';
	}

	async run(key: Key, request: () => Promise<CsvExportResult>, words: (filename: string, count: number) => string) {
		this.isExporting = true;
		this.refusal = '';
		this.#downloaded = undefined;
		let result: CsvExportResult;
		try {
			result = await request();
		} finally {
			this.isExporting = false;
		}
		if ('refusal' in result) {
			this.refusal = result.refusal;
			return;
		}
		saveFile(result.blob, result.filename);
		this.#downloaded = { key, words: words(result.filename, result.count) };
	}
}
