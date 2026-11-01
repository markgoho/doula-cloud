<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { FormError } from '#lib/authRefusal.js';
	import { freshIdToken } from '#lib/firebase.js';
	import {
		owners,
		readLegalVersionCheck,
		sendLegalPublication,
		type LegalPublication,
		type LegalVersionCheck
	} from '#lib/legalVersions.js';
	import { openedOn } from '#lib/pilotGate.js';
	import { longDate } from '#lib/refund.js';
	import { readSession, type Operator } from '#lib/session.js';
	import { stepUp } from '#lib/stepUp.js';
	import ErrorSummary from '#lib/components/ErrorSummary.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';
	import SummaryList from '#lib/components/SummaryList.svelte';

	const document_ = $derived(page.params.document ?? '');
	const version = $derived(page.params.version ?? '');

	let operator = $state<Operator | undefined>();
	let check = $state<LegalVersionCheck | undefined>();
	let result = $state<LegalPublication | undefined>();
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
		const outcome = await readLegalVersionCheck(document_, version);
		if (outcome.kind === 'signed-out') return signedOut();
		if (outcome.kind === 'ok') check = outcome.value;
		else if (outcome.kind === 'refused') errors = outcome.errors;
	});

	async function publish(event: SubmitEvent) {
		event.preventDefault();
		isSending = true;
		try {
			errors = await stepUp({ email: operator!.email, password, code }, { freshIdToken });
			if (errors.length > 0) return;
			const outcome = await sendLegalPublication(document_, version, requestKey);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'step-up') {
				errors = [{ message: 'Enter the code your authenticator app shows now to do this.', field: 'step-up-code' }];
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

<svelte:head><title>Publish a legal version – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if result}
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
{:else if check?.publishedAt}
	<stack-l>
		<h1 tabindex="-1">This version is published</h1>
		<p>
			The {check.documentName} that takes effect {longDate(check.version)} was published on {openedOn(
				check.publishedAt
			)}. A version is published once.
		</p>
	</stack-l>
	<p><a href={resolve('/legal-versions')}>Back to legal versions</a></p>
{:else if check}
	<h1 tabindex="-1">Check the version before you publish it</h1>
	<SummaryList
		rows={[
			{ key: 'Document', value: check.documentName },
			{ key: 'Version', value: check.change },
			{ key: 'Material', value: check.material ? 'Yes' : 'No' },
			{ key: 'Owners emailed', value: owners(check.material, check.owners) },
			{ key: 'Takes effect', value: longDate(check.version) }
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
			<StepUpFields {errors} bind:password bind:code />
			<button class="ops-button" type="submit" disabled={isSending}>Publish</button>
		</stack-l>
	</form>
{:else if errors.length > 0}
	<h1 tabindex="-1">Publish a legal version</h1>
	<p><a href={resolve('/legal-versions')}>Back to legal versions</a></p>
{/if}

<style>
	h1:focus {
		outline: none;
	}
</style>
