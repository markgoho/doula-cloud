<script lang="ts">
	import {
		getMultiFactorResolver,
		signInWithEmailAndPassword,
		signOut,
		TotpMultiFactorGenerator,
		type MultiFactorError,
		type MultiFactorResolver,
		type UserCredential
	} from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getOperatorAuth } from '#lib/firebase.js';
	import { exchangeToken } from '#lib/session.js';
	import {
		codeFieldError,
		codeRefusal,
		credentialRefusal,
		isSecondFactorRequired,
		type FormError
	} from '#lib/authRefusal.js';
	import ErrorSummary from '#lib/components/ErrorSummary.svelte';
	import TextField from '#lib/components/TextField.svelte';

	const emailId = 'email';
	const passwordId = 'password';
	const codeId = 'code';

	let email = $state('');
	let password = $state('');
	let code = $state('');
	let step = $state<'credentials' | 'challenge'>('credentials');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let resolver: MultiFactorResolver | undefined;

	const errorFor = (field: string) => errors.find((error) => error.field === field)?.message;

	async function submit(work: () => Promise<FormError[]>) {
		isSubmitting = true;
		try {
			errors = await work();
		} finally {
			isSubmitting = false;
		}
	}

	async function finish(credential: UserCredential): Promise<FormError[]> {
		const result = await exchangeToken(await credential.user.getIdToken());
		if (result.kind === 'enroll') {
			await goto(resolve('/enroll'));
			return [];
		}
		await signOut(getOperatorAuth());
		if (result.kind === 'refused') return [{ message: result.message }];
		await goto(resolve('/'));
		return [];
	}

	function signIn(event: SubmitEvent) {
		event.preventDefault();
		void submit(async () => {
			const empty: FormError[] = [];
			if (email.trim() === '') empty.push({ message: 'Enter your email address', field: emailId });
			if (password === '') empty.push({ message: 'Enter your password', field: passwordId });
			if (empty.length > 0) return empty;
			try {
				return await finish(await signInWithEmailAndPassword(getOperatorAuth(), email.trim(), password));
			} catch (error) {
				if (!isSecondFactorRequired(error)) return [credentialRefusal(error, passwordId)];
				resolver = getMultiFactorResolver(getOperatorAuth(), error as MultiFactorError);
				step = 'challenge';
				return [];
			}
		});
	}

	function answerChallenge(event: SubmitEvent) {
		event.preventDefault();
		void submit(async () => {
			const empty = codeFieldError(code, codeId);
			if (empty) return [empty];
			try {
				const assertion = TotpMultiFactorGenerator.assertionForSignIn(
					resolver!.hints[0].uid,
					code.replaceAll(/\s/g, '')
				);
				return await finish(await resolver!.resolveSignIn(assertion));
			} catch (error) {
				return [codeRefusal(error, codeId)];
			}
		});
	}
</script>

<svelte:head><title>Sign in – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if step === 'credentials'}
	<h1>Sign in</h1>
	<p>For DoulaCloud's own operator logins.</p>
	<form novalidate onsubmit={signIn}>
		<stack-l>
			<TextField
				id={emailId}
				label="Email address"
				type="email"
				autocomplete="username"
				spellcheck="false"
				bind:value={email}
				error={errorFor(emailId)}
			/>
			<TextField
				id={passwordId}
				label="Password"
				type="password"
				autocomplete="current-password"
				bind:value={password}
				error={errorFor(passwordId)}
			/>
			<button class="ops-button" type="submit" disabled={isSubmitting}>Sign in</button>
		</stack-l>
	</form>
{:else}
	<h1>Enter the code from your authenticator app</h1>
	<form novalidate onsubmit={answerChallenge}>
		<stack-l>
			<TextField
				id={codeId}
				label="Authenticator app code"
				hint="The 6-digit code your app shows for DoulaCloud operations."
				inputmode="numeric"
				autocomplete="one-time-code"
				spellcheck="false"
				bind:value={code}
				error={errorFor(codeId)}
			/>
			<button class="ops-button" type="submit" disabled={isSubmitting}>Continue</button>
		</stack-l>
	</form>
{/if}
