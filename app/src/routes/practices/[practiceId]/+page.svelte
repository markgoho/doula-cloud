<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { registerPushSubscription } from '#lib/pushRegistration.js';

	let practiceName = $state('');
	let roles = $state<string[]>([]);
	let error = $state('');

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/login'));
			return;
		}

		const idToken = await user.getIdToken();
		const response = await apiFetch(`/api/practices/${page.params.practiceId}/session`, idToken);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		const body: { practiceName: string; roles: string[] } = await response.json();
		practiceName = body.practiceName;
		roles = body.roles;

		// Fire-and-forget: #61's "once per device after login" push
		// registration is best-effort and must never block landing on the
		// Practice page (see pushRegistration.ts's doc comment).
		void registerPushSubscription(`/api/practices/${page.params.practiceId}/push-subscriptions`, idToken);
	});
</script>

{#if error}
	<p role="alert">{error}</p>
{:else if practiceName}
	<h1>Welcome to {practiceName}</h1>
	<a href={resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! })}
		>Clients</a
	>
	<a href={resolve('/practices/[practiceId]/billing', { practiceId: page.params.practiceId! })}
		>Billing</a
	>
	{#if roles.includes('owner')}
		<a href={resolve('/practices/[practiceId]/invite', { practiceId: page.params.practiceId! })}
			>Invite a Staff member</a
		>
		<a
			href={resolve('/practices/[practiceId]/settings/plan-templates', {
				practiceId: page.params.practiceId!
			})}>Plan Templates</a
		>
		<a
			href={resolve('/practices/[practiceId]/settings/contract-template', {
				practiceId: page.params.practiceId!
			})}>Contract Template</a
		>
	{/if}
	<a
		href={resolve('/practices/[practiceId]/settings/payments', {
			practiceId: page.params.practiceId!
		})}>Payments</a
	>
{/if}
