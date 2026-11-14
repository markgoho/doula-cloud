<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { cannotBeReachedDraft } from '#lib/cannotBeReachedDraft.svelte.js';

	let { children }: { children: Snippet } = $props();

	cannotBeReachedDraft.start(page.params.practiceId!);
</script>

{@render children()}
