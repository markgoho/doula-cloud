<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { CheckAct } from '#lib/checkAct.svelte.js';
	import { CheckRead } from '#lib/checkRead.svelte.js';
	import { freshIdToken } from '#lib/firebase.js';
	import {
		owners,
		readLegalVersionCheck,
		sendLegalPublication,
		type LegalPublication,
		type LegalVersionCheck
	} from '#lib/legalVersions.js';
	import { newYorkDay } from '#lib/dates.js';
	import { longDate } from '#lib/dates.js';
	import { type Operator } from '#lib/session.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	const document_ = $derived(page.params.document ?? '');
	const version = $derived(page.params.version ?? '');

	const operator = $derived(page.data.operator as Operator);

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	const read = new CheckRead<LegalVersionCheck>(signedOut);
	const act = new CheckAct<LegalPublication>({ onsignedout: signedOut, stepUp: { email: () => operator.email, freshIdToken } });
	const errors = $derived(read.state.status === 'failed' ? read.errors : act.errors);

	onMount(() => read.load(() => readLegalVersionCheck(document_, version)));

	async function publish(event: SubmitEvent) {
		event.preventDefault();
		if (!(await act.run((key) => sendLegalPublication(document_, version, key)))) return;
		await tick();
		document.querySelector('h1')?.focus();
	}
</script>

<svelte:head><title>Publish a legal version – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if act.state.status === 'done'}
	{@const result = act.state.value}
	<stack-l role="status">
		<h1 tabindex="-1">Version published</h1>
		<p>
			The {result.documentName} that takes effect {longDate(result.version)} is published.
			{#if result.material}
				{owners(true, result.queued)}
				{result.queued === 1 ? 'is' : 'are'} emailed about it.
			{:else}
				It is not material, so no one is emailed.
			{/if}
			The Operator log shows the publication under your name.
		</p>
	</stack-l>
	<p><a href={resolve('/legal-versions')}>Back to legal versions</a></p>
{:else if read.state.status === 'failed'}
	<h1 tabindex="-1">Publish a legal version</h1>
	<p><a href={resolve('/legal-versions')}>Back to legal versions</a></p>
{:else if read.state.status === 'loaded'}
	{@const check = read.state.value}
	{#if check.publishedAt}
		<stack-l>
			<h1 tabindex="-1">This version is published</h1>
			<p>
				The {check.documentName} that takes effect {longDate(check.version)} was published on {newYorkDay(
					check.publishedAt
				)}. A version is published once.
			</p>
		</stack-l>
		<p><a href={resolve('/legal-versions')}>Back to legal versions</a></p>
	{:else}
		<h1 tabindex="-1">Check the version before you publish it</h1>
		<SummaryList
			rows={[
				{ label: 'Document', value: check.documentName },
				{ label: 'Version', value: check.change },
				{ label: 'Material', value: check.material ? 'Yes' : 'No' },
				{ label: 'Owners emailed', value: owners(check.material, check.owners) },
				{ label: 'Takes effect', value: longDate(check.version) }
			]}
		/>
		<form novalidate onsubmit={publish}>
			<stack-l>
				<p>
					{#if check.material}
						Each Owner is emailed once, now, about this version. A publication cannot be undone.
					{:else}
						A publication cannot be undone.
					{/if}
				</p>
				<StepUpFields {errors} bind:password={act.password} bind:code={act.code} />
				<Button label="Publish" type="submit" loading={act.isSending} />
			</stack-l>
		</form>
	{/if}
{/if}

<style>
	h1:focus {
		outline: none;
	}
</style>
