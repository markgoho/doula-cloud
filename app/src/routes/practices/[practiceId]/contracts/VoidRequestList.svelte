<script lang="ts">
	import { voidRequestAsked, type AwaitingVoidRequest } from '#lib/contract.js';
	import Link from '#lib/components/atoms/Link.svelte';

	let {
		requests,
		hrefFor
	}: {
		requests: AwaitingVoidRequest[];
		hrefFor: (request: AwaitingVoidRequest) => string;
	} = $props();
</script>

<div class="frame">
	<ul>
		{#each requests as request (request.requestId)}
			<li id="void-request-row-{request.requestId}">
				<div class="title"><Link href={hrefFor(request)} label={request.clientName} ariaLabel={`${request.clientName}, void request`} /></div>
				<p class="meta">{request.requestedByName} {voidRequestAsked(request.daysAgo)}</p>
				<p class="reason">{request.reason}</p>
			</li>
		{/each}
	</ul>
</div>

<style>
	@layer components {
		.frame {
			container: void-requests / inline-size;
			align-self: stretch;
		}

		ul {
			margin: 0;
			padding: 0;
			list-style: none;
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(min(100%, 36ch), 1fr));
			gap: var(--space-related);
		}

		li {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
			min-inline-size: 0;
			padding: var(--space-3) var(--space-4);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			scroll-margin-block-start: calc(var(--top-bar-height) + var(--space-4));
		}

		p {
			margin: 0;
			overflow-wrap: anywhere;
		}

		.title {
			min-inline-size: 0;
			overflow-wrap: anywhere;
			font-size: var(--text-subheading-size);
			font-weight: var(--text-subheading-weight);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
		}

		.meta {
			font-size: var(--text-body-sm-size);
			line-height: var(--text-body-sm-leading);
			color: var(--color-on-surface-variant);
		}

		.reason {
			font-size: var(--text-body-size);
		}
	}
</style>
