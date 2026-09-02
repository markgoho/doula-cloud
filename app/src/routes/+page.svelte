<script lang="ts">
	import { resolve } from '$app/paths';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
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
	{#if data.memberships.length === 0}
		<p>You don't belong to any Practice yet. Ask an Owner to invite you.</p>
	{:else}
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
	{/if}
{:else}
	<Heading level={1} text="Choose an Engagement" />
	{#if data.engagements.length === 0}
		<p>You don't have an Engagement yet. Ask your Practice to set one up.</p>
	{:else}
		<ul>
			{#each data.engagements as engagement (engagement.engagementId)}
				<li>
					<Link
						href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
						label={engagement.practiceName}
					/>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
