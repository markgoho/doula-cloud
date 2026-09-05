<script lang="ts">
	/*
	 * TOTP enrolment (#606). Reached two ways: driven here by
	 * `apiFetchWithSession`'s `MFA_REQUIRED` redirect (#lib/api.js), with
	 * `returnTo` naming the Practice-scoped page that refused her, or
	 * navigated to voluntarily from account settings, hours into an
	 * ordinary session, with no `returnTo` at all.
	 *
	 * Decision 2 (issue #606's triage brief) settles the sign-out
	 * collision #149 and this ticket both have a claim on: enrolment
	 * always re-authenticates rather than ever assuming the browser holds
	 * a live client-side Identity Platform sign-in. That holds on both
	 * entry points -- the BFF session cookie #149 signs out from under is
	 * a different thing from the JS SDK's own signed-in user, and by the
	 * time either kind of visit reaches this screen the SDK has already
	 * been signed out (#149 fires right after every session exchange this
	 * app makes). So step one always asks for the password again; there is
	 * no branch on `getFirebaseAuth().currentUser` to skip it.
	 *
	 * Two steps, never rendered together, the same shape `accept-invite`
	 * already uses for its own two-step form: step one re-authenticates
	 * and opens an enrolment session with Identity Platform, step two
	 * shows the QR code and secret and asks for the code it produces.
	 */
	import { onMount } from 'svelte';
	import {
		multiFactor,
		signInWithEmailAndPassword,
		signOut,
		TotpMultiFactorGenerator,
		type User
	} from 'firebase/auth';
	import QRCode from 'qrcode';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, probeSession } from '#lib/api.js';
	import {
		passwordReauthRefusal,
		refusalMessage,
		totpCodeRefusal,
		type FormError
	} from '#lib/formErrors.js';
	import type { SessionInfo } from '#lib/landing.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TotpCodeField from '#lib/components/molecules/TotpCodeField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const passwordId = 'mfa-enroll-password';
	const codeId = 'mfa-enroll-code';

	let step = $state<'password' | 'setup'>('password');
	let email = $state('');
	let password = $state('');
	let code = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let qrCodeDataUrl = $state('');
	let secretKey = $state('');

	/*
	 * Held across the two steps without being `$state`: neither is ever
	 * read by the markup, only by the handler that runs after the one that
	 * set it -- the same reason `accept-invite` keeps its own step-one
	 * `credential` as a plain variable.
	 */
	let enrollingUser: User | undefined;
	let totpSecret: Awaited<ReturnType<typeof TotpMultiFactorGenerator.generateSecret>> | undefined;

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	onMount(async () => {
		const session = await probeSession<SessionInfo>('/api/staff/session');
		if (!session) {
			await goto(resolve('/(signed-out)/login'));
			return;
		}
		email = session.email;
	});

	function signOutOfFirebaseSDK() {
		void signOut(getFirebaseAuth());
	}

	// #167's shared-device concern: a live client-side Identity Platform
	// sign-in must not survive her navigating away mid-flow, success or
	// abandonment alike. Every exit this component has -- both handlers'
	// own `signOut` calls included -- is a no-op by the time this runs, so
	// there is nothing to guard; the guard is here for the exits that skip
	// them, chiefly leaving the page.
	onMount(() => signOutOfFirebaseSDK);

	function safeReturnTo(): string | undefined {
		const target = page.url.searchParams.get('returnTo');
		return target && target.startsWith('/') && !target.startsWith('//') ? target : undefined;
	}

	async function landAfterEnrolment(): Promise<void> {
		await goto(safeReturnTo() ?? resolve('/'));
	}

	async function handlePasswordSubmit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		errors = [];

		if (password === '') {
			errors = [{ message: 'Enter your password', targetId: passwordId }];
			return;
		}

		isSubmitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			enrollingUser = credential.user;

			const session = await multiFactor(enrollingUser).getSession();
			totpSecret = await TotpMultiFactorGenerator.generateSecret(session);
			qrCodeDataUrl = await QRCode.toDataURL(totpSecret.generateQrCodeUrl(email, 'Doula Cloud'));
			secretKey = totpSecret.secretKey;
			step = 'setup';
		} catch (error_) {
			errors = [passwordReauthRefusal(error_, passwordId)];
		} finally {
			isSubmitting = false;
		}
	}

	async function handleCodeSubmit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		errors = [];

		if (code.trim() === '') {
			errors = [{ message: 'Enter the 6-digit code from your authenticator app', targetId: codeId }];
			return;
		}

		isSubmitting = true;
		try {
			const assertion = TotpMultiFactorGenerator.assertionForEnrollment(totpSecret!, code);
			await multiFactor(enrollingUser!).enroll(assertion, 'Authenticator app');

			/*
			 * #613/#169's own discipline, repeated here per #606's brief: the
			 * SDK's cached token predates the claim `enroll()` just added, so
			 * the BFF must be handed a freshly minted one or it reads a token
			 * that does not show the second factor yet.
			 */
			const idToken = await enrollingUser!.getIdToken(true);

			const response = await fetch(`${apiBaseURL()}/api/staff/mfa`, {
				method: 'POST',
				credentials: 'include',
				headers: { Authorization: `Bearer ${idToken}` }
			});

			if (response.ok) {
				await signOut(getFirebaseAuth());
				await landAfterEnrolment();
				return;
			}

			if (response.status === 400) {
				/*
				 * Decision 4: the post-enrolment token turned out not to carry
				 * the claim yet. This is expected fallback plumbing, not a
				 * form refusal -- the ordinary sign-in flow's TOTP challenge
				 * mints a session that does show it.
				 */
				await signOut(getFirebaseAuth());
				await goto(resolve('/(signed-out)/login'));
				return;
			}

			errors = [{ message: await refusalMessage(response) }];
		} catch (error_) {
			errors = [totpCodeRefusal(error_, codeId)];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet content()}
	{#if step === 'password'}
		<!-- `novalidate`: the page refuses the submit, not the browser (#467). -->
		<form onsubmit={handlePasswordSubmit} novalidate>
			<Text text="Confirm your password to set up an authenticator app." />
			<LabeledField id={passwordId} label="Password" error={errorFor(passwordId)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="password"
						value={password}
						onInput={(value) => (password = value)}
						required
						autocomplete="current-password"
					/>
				{/snippet}
			</LabeledField>
			<Button type="submit" label="Continue" loading={isSubmitting} />
		</form>
	{:else}
		<form onsubmit={handleCodeSubmit} novalidate>
			<Text
				text="Scan this QR code with an authenticator app, such as Google Authenticator or 1Password."
			/>
			<img
				src={qrCodeDataUrl}
				alt="QR code for setting up two-factor authentication in an authenticator app"
				width="200"
				height="200"
			/>
			<Text text="Can't scan the code? Enter this key into your authenticator app instead:" />
			<!--
				Selectable plain text, not an input: a person enrolling on the
				same device she is reading the screen on cannot scan her own
				screen, so this is the one alternative path -- #606's own AC.
			-->
			<p><code>{secretKey}</code></p>
			<TotpCodeField id={codeId} value={code} onInput={(value) => (code = value)} error={errorFor(codeId)} />
			<Button type="submit" label="Confirm and turn on" loading={isSubmitting} />
		</form>
	{/if}
{/snippet}

<EntryPage
	title="Set up two-factor authentication"
	errorSummary={errors.length > 0 ? errorSummary : undefined}
	{content}
/>
