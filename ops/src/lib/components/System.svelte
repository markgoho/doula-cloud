<script lang="ts">
	import { onMount } from 'svelte';
	import type { FormError } from '../authRefusal.ts';
	import {
		arrivalLabel,
		modeLabel,
		outboxLabel,
		readSystem,
		refusalBreakdown,
		refusalTotal,
		routeLabel,
		typeArrivalLabel,
		undeliveredLabel,
		type System
	} from '../system.ts';
	import { sentAt } from '../feedback.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let { onsignedout, fetcher = fetch }: { onsignedout: () => void; fetcher?: typeof fetch } = $props();

	let read = $state<System | undefined>();
	let errors = $state<FormError[]>([]);

	const NOT_READ = 'Stripe did not answer';

	onMount(async () => {
		const outcome = await readSystem(fetcher);
		if (outcome.kind === 'signed-out') return onsignedout();
		if (outcome.kind === 'refused') {
			errors = outcome.errors;
			return;
		}
		/* v8 ignore next -- a read asks no fresh code, so 'step-up' never arrives */
		if (outcome.kind === 'ok') read = outcome.value;
	});
</script>

<ErrorSummary {errors} />

{#if read}
	<!-- v8 ignore start: Svelte's compiled update branches for each value; a read is set once and never changes in place -->
	<stack-l>
		<section>
			<stack-l>
				<h2>Stripe</h2>
				<SummaryList
					rows={[
						{ label: 'Mode', value: modeLabel(read.stripe.mode) },
						{ label: 'Platform account', value: read.stripe.platformAccountId ?? NOT_READ },
						{
							label: 'Events Stripe could not deliver in the last 7 days',
							value: read.stripe.undelivered ? undeliveredLabel(read.stripe.undelivered) : NOT_READ
						}
					]}
				/>
			</stack-l>
		</section>

		<section>
			<stack-l>
				<h2>Webhooks</h2>
				<div class="scroll">
					<table>
						<caption>Each route’s last arrival and its refusals in the last 7 days</caption>
						<thead>
							<tr>
								<th scope="col">Route</th>
								<th scope="col">Last arrival</th>
								<th scope="col">Refused</th>
							</tr>
						</thead>
						<tbody>
							{#each read.webhooks as route (route.route)}
								<tr>
									<th scope="row">{routeLabel(route.route)}</th>
									<td>{arrivalLabel(route.lastArrivalAt)}</td>
									<td>
										{refusalTotal(route.refusals)}
										{#if refusalTotal(route.refusals) > 0}
											<span class="detail">({refusalBreakdown(route.refusals)})</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</stack-l>
		</section>

		<section>
			<stack-l>
				<h2>Stripe event types</h2>
				<div class="scroll">
					<table>
						<caption>The last arrival of each event type DoulaCloud handles</caption>
						<thead>
							<tr>
								<th scope="col">Event type</th>
								<th scope="col">Route</th>
								<th scope="col">Last arrival</th>
							</tr>
						</thead>
						<tbody>
							{#each read.eventTypes as event (`${event.route}/${event.type}`)}
								<tr>
									<th scope="row" class="code">{event.type}</th>
									<td>{routeLabel(event.route)}</td>
									<td>{typeArrivalLabel(event.lastArrivalAt)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</stack-l>
		</section>

		<section>
			<stack-l>
				<h2>Outboxes</h2>
				<div class="scroll">
					<table>
						<caption>Rows waiting, sent, skipped with nothing to send, and failed for good</caption>
						<thead>
							<tr>
								<th scope="col">Outbox</th>
								<th scope="col" class="number">Pending</th>
								<th scope="col" class="number">Sent</th>
								<th scope="col" class="number">Skipped</th>
								<th scope="col" class="number">Dead-lettered</th>
								<th scope="col" class="number">Bounced</th>
								<th scope="col" class="number">Marked as spam</th>
								<th scope="col">Oldest due</th>
							</tr>
						</thead>
						<tbody>
							{#each read.outboxes as outbox (outbox.name)}
								<tr>
									<th scope="row">{outboxLabel(outbox.name)}</th>
									<td class="number">{outbox.pending}</td>
									<td class="number">{outbox.sent}</td>
									<td class="number">{outbox.skipped}</td>
									<td class="number">{outbox.deadLettered}</td>
									<td class="number">{outbox.bounced}</td>
									<td class="number">{outbox.complained}</td>
									<td>{outbox.oldestDueAt ? sentAt(outbox.oldestDueAt) : 'None due'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</stack-l>
		</section>

		<section>
			<stack-l>
				<h2>Pilot gate</h2>
				{#if read.pilot.gate.openedAt}
					<p>
						Open since {sentAt(read.pilot.gate.openedAt)}{read.pilot.gate.openedBy
							? `, opened by ${read.pilot.gate.openedBy}`
							: ''}.
					</p>
				{:else}
					<p>Closed. Only an address on the allowlist can create a Practice.</p>
				{/if}
			</stack-l>
		</section>

		<section>
			<stack-l>
				<h2>Pilot allowlist</h2>
				{#if read.pilot.allowlist.items.length === 0}
					<p>No address is on the allowlist.</p>
				{:else}
					{#if read.pilot.allowlist.total > read.pilot.allowlist.items.length}
						<p>The newest {read.pilot.allowlist.items.length} of {read.pilot.allowlist.total} addresses.</p>
					{/if}
					<div class="scroll">
						<table>
							<caption>Addresses that can create a Practice while the gate is closed</caption>
							<thead>
								<tr>
									<th scope="col">Address</th>
									<th scope="col">Added</th>
									<th scope="col">Added by</th>
								</tr>
							</thead>
							<tbody>
								{#each read.pilot.allowlist.items as item (item.address)}
									<tr>
										<th scope="row">{item.address}</th>
										<td>{sentAt(item.addedAt)}</td>
										<td>{item.addedBy ?? 'Not recorded'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</stack-l>
		</section>
	</stack-l>
	<!-- v8 ignore stop -->
{/if}

<style>
	.scroll {
		max-inline-size: 100%;
		overflow-x: auto;
	}

	table {
		inline-size: 100%;
		border-collapse: collapse;
	}

	caption {
		padding-block-end: var(--space-2);
		font-weight: bold;
		text-align: start;
	}

	th,
	td {
		padding: var(--space-2);
		text-align: start;
		vertical-align: top;
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	tbody th {
		font-weight: normal;
	}

	.code {
		overflow-wrap: anywhere;
		font-family: var(--font-mono, monospace);
	}

	.number {
		text-align: end;
		font-variant-numeric: tabular-nums;
	}

	.detail {
		color: var(--color-on-surface-muted);
	}
</style>
