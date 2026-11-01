<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { FormError } from '#lib/authRefusal.js';
	import { readLegalVersions, type LegalVersion } from '#lib/legalVersions.js';
	import { newYorkDay } from '#lib/dates.js';
	import { longDate } from '#lib/dates.js';
		import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';

	let versions = $state<LegalVersion[] | undefined>();
	let errors = $state<FormError[]>([]);

	onMount(async () => {
		const outcome = await readLegalVersions();
		if (outcome.kind === 'signed-out') return goto(resolve('/sign-in'));
		if (outcome.kind === 'ok') versions = outcome.value;
		else if (outcome.kind === 'refused') errors = outcome.errors;
	});
</script>

<svelte:head><title>Legal versions – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

<h1>Legal versions</h1>
<p>
	Each version of the Terms of Service and the Privacy Policy in the history, newest first. A version is written into
	the history and onto the site first, then published here.
</p>

{#if versions}
	<ul role="list">
		{#each versions as version (`${version.document} ${version.version}`)}
			<li>
				<stack-l>
					<h2>{version.documentName}, takes effect {longDate(version.version)}</h2>
					<p>{version.change}</p>
					{#if version.publishedAt}
						<p>Published on {newYorkDay(version.publishedAt)}.</p>
					{:else}
						<p>
							<a href={resolve('/legal-versions/[document]/[version]', { document: version.document, version: version.version })}
								>Check and publish<span class="visually-hidden">
									the {version.documentName} that takes effect {longDate(version.version)}</span
								></a
							>
						</p>
					{/if}
				</stack-l>
			</li>
		{/each}
	</ul>
{/if}

<style>
	ul {
		max-inline-size: var(--measure);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	li {
		padding-block: var(--space-3);
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	h2 {
		margin: 0;
		font-size: inherit;
	}
</style>
