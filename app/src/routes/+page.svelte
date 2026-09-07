<script lang="ts">
	import { resolve } from '$app/paths';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import { CARE_HEADING, NO_CARE_MESSAGE, engagementLabel, engagementStatusLabel } from '#lib/clientRegister.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	// Mirrors StepRail's own id-per-row join (#464): the status text sits
	// beside the link rather than inside its accessible name, so a
	// keyboard or screen-reader user still hears the status without it
	// being read as part of the link's own name. Telling two Engagements
	// at the same Practice apart is the link's own name's job instead
	// (#310): engagementLabel folds in when each began.
	function statusId(engagementId: string) {
		return `engagement-status-${engagementId}`;
	}
</script>

<PageTitle page="Home" />

{#if data.type === 'signed-out'}
	<Heading level={1} text="Doula Cloud" />
	<ul>
		<li><Link href={resolve('/(signed-out)/login')} label="Staff log in" /></li>
		<li><Link href={resolve('/(signed-out)/signup')} label="Set up a Practice" /></li>
		<li><Link href={resolve('/portal/(signed-out)/login')} label="Client portal log in" /></li>
	</ul>
{:else if data.type === 'staff-picker'}
	<Heading level={1} text="Choose a Practice" />
	<!--
		Always at least one: `+page.ts` redirects a Staff member with no
		Membership to `/no-practice` rather than handing this an empty list
		(#745).
	-->
	<ul>
		{#each data.memberships as membership (membership.practiceId)}
			<li>
				<Link
					href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}
					label={membership.practiceName}
				/>
			</li>
		{/each}
	</ul>
{:else}
	<Heading level={1} text={CARE_HEADING} />
	{#if data.engagements.length === 0}
		<p>{NO_CARE_MESSAGE}</p>
	{:else}
		<!--
			#312: every Engagement her Portal Account reaches, across every
			Practice, past and present -- a completed one stays listed and
			stays openable (ADR-0015). `+page.ts` already redirects straight
			through when there is only one, so this only ever renders with
			two or more.
		-->
		<ul>
			{#each data.engagements as engagement (engagement.engagementId)}
				<li>
					<Link
						href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
						label={engagementLabel(engagement)}
						describedBy={statusId(engagement.engagementId)}
					/>
					<p class="status" id={statusId(engagement.engagementId)}>
						{engagementStatusLabel(engagement.status)}
					</p>
				</li>
			{/each}
		</ul>
	{/if}
{/if}

<style>
	@layer components {
		.status {
			margin: 0 0 var(--space-3);
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
