<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { CheckAct } from '#lib/checkAct.svelte.js';
	import { CheckRead } from '#lib/checkRead.svelte.js';
	import { freshIdToken } from '#lib/firebase.js';
	import { type Operator } from '#lib/session.js';
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

	const operator = $derived(page.data.operator as Operator);

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	const read = new CheckRead<SecondFactorStaff>(signedOut);
	const act = new CheckAct<SecondFactorStaff>({ onsignedout: signedOut, stepUp: { email: () => operator.email, freshIdToken } });
	const errors = $derived(read.state.status === 'failed' ? read.errors : act.errors);

	onMount(() => read.load(() => readSecondFactorCheck(staffId)));

	async function reset(event: SubmitEvent) {
		event.preventDefault();
		if (!(await act.run((key) => sendSecondFactorClear(staffId, key)))) return;
		await tick();
		document.querySelector('h1')?.focus();
	}
</script>

<svelte:head><title>Reset two-factor authentication – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if act.state.status === 'done'}
	{@const result = act.state.value}
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
{:else if read.state.status === 'failed'}
	<h1 tabindex="-1">Reset two-factor authentication</h1>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'loaded'}
	{@const check = read.state.value}
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
			<StepUpFields {errors} bind:password={act.password} bind:code={act.code} />
			<p>
				Every authenticator app on this sign-in is removed, every device is signed out, and an email says so at once. A
				reset cannot be undone.
			</p>
			<Button label="Reset two-factor authentication" type="submit" loading={act.isSending} />
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
