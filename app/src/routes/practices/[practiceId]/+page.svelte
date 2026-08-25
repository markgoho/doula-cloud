<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetch, apiFetchWithSession } from '#lib/api.js';
	import {
		practicePushSubscriptionsPath,
		registerPushSubscription
	} from '#lib/pushRegistration.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	let practiceName = $state('');
	let roles = $state<string[]>([]);
	let error = $state('');

	onMount(async () => {
		const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/session`);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		const body: { practiceName: string; roles: string[] } = await response.json();
		practiceName = body.practiceName;
		roles = body.roles;

		// Fire-and-forget: #61's "once per device after login" push
		// registration is best-effort and must never block landing on the
		// Practice page, or navigate the page away, on any failure (see
		// pushRegistration.ts's doc comment) -- apiFetch, not
		// apiFetchWithSession, since that helper's own 401 handling would
		// sign the person out and redirect on a failure this call is
		// supposed to swallow silently.
		void registerPushSubscription(
			practicePushSubscriptionsPath(page.params.practiceId!),
			apiFetch
		);
	});
</script>

{#if error}
	<Notice variant="error" message={error} />
{:else if practiceName}
	<Heading level={1} text={`Welcome to ${practiceName}`} />
	<Link
		href={resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! })}
		label="Clients"
	/>
	<Link
		href={resolve('/practices/[practiceId]/billing', { practiceId: page.params.practiceId! })}
		label="Billing"
	/>
	<!-- Everyone's own Offers, not only a Doula's: an Offer is addressed to
	     a person, and the inbox is scoped to her staff id, so there is
	     nothing here a role check would usefully hide. -->
	<Link
		href={resolve('/practices/[practiceId]/offers', { practiceId: page.params.practiceId! })}
		label="Your offers"
	/>
	{#if roles.includes('owner')}
		<Link
			href={resolve('/practices/[practiceId]/invite', { practiceId: page.params.practiceId! })}
			label="Invite a Staff member"
		/>
		<Link
			href={resolve('/practices/[practiceId]/staff', { practiceId: page.params.practiceId! })}
			label="Staff"
		/>
		<Link
			href={resolve('/practices/[practiceId]/settings/plan-templates', {
				practiceId: page.params.practiceId!
			})}
			label="Plan Templates"
		/>
		<Link
			href={resolve('/practices/[practiceId]/settings/contract-template', {
				practiceId: page.params.practiceId!
			})}
			label="Contract Template"
		/>
	{/if}
	<Link
		href={resolve('/practices/[practiceId]/settings/payments', {
			practiceId: page.params.practiceId!
		})}
		label="Payments"
	/>
{/if}
