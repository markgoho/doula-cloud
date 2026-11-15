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
				<p>
					<span><Link href={hrefFor(request)} label={request.clientName} ariaLabel={`${request.clientName}, void request`} /></span>
					<span aria-hidden="true" class="joiner"> · </span><span class="visually-hidden">, </span>
					<span>{request.requestedByName} {voidRequestAsked(request.daysAgo)}:</span>
					<span class="reason">{request.reason}</span>
				</p>
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
			font-size: var(--text-body-size);
			margin: 0;
			padding: 0;
			list-style: none;
			display: flex;
			flex-direction: column;
			gap: 0;
		}

		p {
			margin: 0;
			overflow-wrap: anywhere;
		}

		@container void-requests (min-width: 60rem) {
			ul {
				display: grid;
				grid-template-columns: minmax(0, max-content) minmax(0, max-content) minmax(0, 72ch);
				column-gap: var(--space-3);
				justify-content: start;
			}

			li {
				display: grid;
				grid-column: 1 / -1;
				grid-template-columns: subgrid;
			}

			p {
				display: contents;
			}

			.joiner {
				display: none;
			}
		}
	}
</style>
