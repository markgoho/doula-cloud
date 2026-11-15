<script lang="ts">
	/*
	 * A row's Doulas on a Staff list, in a table cell or a record card. The
	 * Contracts list and the Clients list draw them with it, so the two say
	 * it the same way (#2958): "You" where the reader is one of them, first;
	 * every name where they fit a one-line cell, and else the first name and
	 * "+N". A screen reader hears every name; a card, which wraps, says them
	 * all.
	 */
	import { DOULA_CELL_FITS, doulaWords } from '#lib/doulaNames.js';

	interface Properties {
		names: readonly string[];
		ids?: readonly string[];
		readerId?: string;
		isTable?: boolean;
		prefix?: string;
	}

	let { names, ids = [], readerId = '', isTable = false, prefix = '' }: Properties = $props();

	const words = $derived(doulaWords(names, ids, readerId, DOULA_CELL_FITS - prefix.length));
	const isFolded = $derived(isTable && words.more > 0);
	const seen = $derived(isFolded ? `${prefix}${words.lead} +${words.more}` : `${prefix}${isTable ? words.lead : words.full}`);
	const heard = $derived(`${prefix}${words.full}`);
</script>

<span>
	{#if isFolded}<span aria-hidden="true">{seen}</span><span class="visually-hidden">{heard}</span>{:else}{seen}{/if}
</span>
