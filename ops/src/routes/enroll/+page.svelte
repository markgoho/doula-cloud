<script lang="ts">
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { multiFactor, signOut, TotpMultiFactorGenerator, type TotpSecret } from 'firebase/auth';
	import QRCode from 'qrcode';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getOperatorAuth } from '#lib/firebase.js';
	import { exchangeToken, SERVICE_PROBLEM } from '#lib/session.js';
	import { codeFieldError, codeRefusal, type FormError } from '#lib/authRefusal.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';

	const codeId = 'code';

	let secret: TotpSecret | undefined;
	let qrCode = $state<string | undefined>();
	let key = $state('');
	let code = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);

	const codeError = $derived(errors.find((error) => error.targetId === codeId)?.message);

	// A login reaches here only from sign-in, which keeps its password
	// sign-in open for this. Opened any other way, there is nothing to
	// enroll against, so sign-in is where to go.
	onMount(async () => {
		const user = getOperatorAuth().currentUser;
		if (!user) {
			await goto(resolve('/sign-in'));
			return;
		}
		try {
			secret = await TotpMultiFactorGenerator.generateSecret(await multiFactor(user).getSession());
			key = secret.secretKey;
			qrCode = await QRCode.toDataURL(secret.generateQrCodeUrl(user.email ?? 'operator', 'DoulaCloud operations'));
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		}
	});

	async function enroll(event: SubmitEvent) {
		event.preventDefault();
		isSubmitting = true;
		try {
			errors = await confirm();
		} finally {
			isSubmitting = false;
		}
	}

	async function confirm(): Promise<FormError[]> {
		const empty = codeFieldError(code, codeId);
		if (empty) return [empty];
		const user = getOperatorAuth().currentUser!;
		try {
			const assertion = TotpMultiFactorGenerator.assertionForEnrollment(secret!, code.replaceAll(/\s/g, ''));
			await multiFactor(user).enroll(assertion, 'Authenticator app');
		} catch (error) {
			return [codeRefusal(error, codeId)];
		}
		const result = await exchangeToken(await user.getIdToken(true));
		await signOut(getOperatorAuth());
		if (result.kind !== 'signed-in') return [{ message: result.kind === 'refused' ? result.message : SERVICE_PROBLEM }];
		await goto(resolve('/'));
		return [];
	}
</script>

<svelte:head><title>Set up an authenticator app – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

<h1>Set up an authenticator app</h1>
<p>Every sign-in to DoulaCloud operations asks for a code from an authenticator app.</p>

<h2>1. Add DoulaCloud operations to your app</h2>
{#if qrCode}
	<img class="qr" src={qrCode} alt="QR code for setting up an authenticator app" width="200" height="200" />
{/if}
{#if key}
	<p>Or enter this key in your app:</p>
	<p class="key">{key}</p>
{/if}

<h2>2. Enter the code your app shows</h2>
<form novalidate onsubmit={enroll}>
	<stack-l>
		<LabeledField id={codeId} label="Authenticator app code" error={codeError}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					inputmode="numeric"
					autocomplete="one-time-code"
					value={code}
					onInput={(next) => (code = next)}
				/>
			{/snippet}
		</LabeledField>
		<Button label="Confirm and sign in" type="submit" loading={isSubmitting} disabled={!key} />
	</stack-l>
</form>

<style>
	.qr {
		inline-size: var(--thumb-max-inline);
		max-inline-size: 100%;
		block-size: auto;
	}

	.key {
		font-family: monospace;
		overflow-wrap: anywhere;
	}
</style>
