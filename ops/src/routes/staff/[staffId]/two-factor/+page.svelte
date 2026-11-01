<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { FormError } from '#lib/authRefusal.js';
	import { freshIdToken } from '#lib/firebase.js';
	import { readSession, type Operator } from '#lib/session.js';
	import { stepUp } from '#lib/stepUp.js';
	import {
		practiceList,
		readSecondFactorCheck,
		sendSecondFactorClear,
		type SecondFactorStaff
	} from '#lib/secondFactor.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	const staffId = $derived(page.params.staffId ?? '');

	let operator = $state<Operator | undefined>();
	let check = $state<SecondFactorStaff | undefined>();
	let result = $state<SecondFactorStaff | undefined>();
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);
	let password = $state('');
	let code = $state('');
	const requestKey = crypto.randomUUID();

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	onMount(async () => {
		operator = await readSession();
		if (!operator) return signedOut();
		const outcome = await readSecondFactorCheck(staffId);
		if (outcome.kind === 'signed-out') return signedOut();
		if (outcome.kind === 'ok') check = outcome.value;
		else if (outcome.kind === 'refused') errors = outcome.errors;
	});

	async function reset(event: SubmitEvent) {
		event.preventDefault();
		isSending = true;
		try {
			errors = await stepUp({ email: operator!.email, password, code }, { freshIdToken });
			if (errors.length > 0) return;
			const outcome = await sendSecondFactorClear(staffId, requestKey);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'step-up') {
				errors = [{ message: 'Enter the code your authenticator app shows now to do this.', targetId: 'step-up-code' }];
				return;
			}
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				return;
			}
			result = outcome.value;
			await tick();
			document.querySelector('h1')?.focus();
		} finally {
			isSending = false;
		}
	}
</script>

<svelte:head><title>Reset two-factor authentication – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if result}
	<stack-l role="status">
		<h1 tabindex="-1">Two-factor authentication reset</h1>
		<p>
			{result.staffName} is signed out of every device and has been emailed. The next sign-in asks for a new authenticator
			app wherever a Practice requires one.
		</p>
		{#if result.practiceNames.length > 0}
			<p>The Activity at {practiceList(result.practiceNames)} shows the reset under your name.</p>
		{/if}
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if check}
	<p class="caption">{check.staffName}</p>
	<h1 tabindex="-1">Check before you reset two-factor authentication</h1>
	<SummaryList
		rows={[
			{ label: 'Staff member', value: check.staffName },
			{ label: 'Practices', value: practiceList(check.practiceNames) }
		]}
	/>
	<stack-l>
		<h2>Reset only when both of these are true</h2>
		<ul class="prose-list">
			<li>You can see this person now, on a live video call.</li>
			<li>
				The government photo ID shown on that call matches the identity-verified representative on the Practice's Stripe
				account.
			</li>
		</ul>
		<p>
			If the Practice's Stripe account was never finished, or its verified representative is someone else, stop. An
			email, a phone call or details about the Practice are not proof.
		</p>
	</stack-l>
	<form novalidate onsubmit={reset}>
		<stack-l>
			<StepUpFields {errors} bind:password bind:code />
			<p>
				Every authenticator app on this sign-in is removed, every device is signed out, and an email says so at once. A
				reset cannot be undone.
			</p>
			<Button label="Reset two-factor authentication" type="submit" loading={isSending} />
		</stack-l>
	</form>
{/if}

<style>
	.caption {
		color: var(--color-on-surface-variant);
	}

	h1:focus {
		outline: none;
	}
</style>
