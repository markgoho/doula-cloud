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
		refusalOrConfirmable,
		totpCodeRefusal,
		type FormError
	} from '#lib/formErrors.js';
	import type { SessionInfo } from '#lib/landing.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TotpCodeField from '#lib/components/molecules/TotpCodeField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const passwordId = 'mfa-enroll-password';
	const codeId = 'mfa-enroll-code';

	// #816: this two-step form has no single Continue button to hang
	// #610's cross-population warning on, so it hangs on step two's own
	// final submit -- "Confirm and turn on" is the deliberate press that
	// mints a session, the same reasoning the sign-in page's Continue
	// button uses.
	let step = $state<'password' | 'setup' | 'confirm-sign-out'>('password');
	let email = $state('');
	let password = $state('');
	let code = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let qrCodeDataUrl = $state('');
	let secretKey = $state('');
	let signOutWarning = $state<string | undefined>();

	/*
	 * Held across the two steps without being `$state`: neither is ever
	 * read by the markup, only by the handler that runs after the one that
	 * set it -- the same reason `accept-invite` keeps its own step-one
	 * `credential` as a plain variable.
	 */
	let enrollingUser: User | undefined;
	let totpSecret: Awaited<ReturnType<typeof TotpMultiFactorGenerator.generateSecret>> | undefined;
	// The freshly-minted, second-factor-showing ID token #816's confirmed
	// retry re-sends -- computed once in handleCodeSubmit rather than
	// re-derived, since a second `getIdToken(true)` call is a second
	// network round trip for no reason the retry needs.
	let pendingIdToken = '';

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

	/*
	 * The POST every enrolment-finish attempt runs, first unconfirmed and
	 * then -- if #816's cross-population check refuses it -- again with
	 * X-Confirmed, on the same idToken.
	 */
	async function postFinishEnrollment(idToken: string, isConfirmed: boolean) {
		return fetch(`${apiBaseURL()}/api/staff/mfa`, {
			method: 'POST',
			credentials: 'include',
			headers: {
				Authorization: `Bearer ${idToken}`,
				...(isConfirmed && { 'X-Confirmed': 'true' })
			}
		});
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
			pendingIdToken = await enrollingUser!.getIdToken(true);

			const response = await postFinishEnrollment(pendingIdToken, false);

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

			// #816: a live portal session in this browser is not a form
			// refusal -- nothing is wrong with the code she entered, and the
			// same finish sent again with X-Confirmed goes through. Keep the
			// JS SDK signed in for that retry.
			const refusal = await refusalOrConfirmable(response);
			if (refusal.kind === 'confirmable') {
				signOutWarning = refusal.message;
				step = 'confirm-sign-out';
				return;
			}
			errors = refusal.errors;
		} catch (error_) {
			errors = [totpCodeRefusal(error_, codeId)];
		} finally {
			isSubmitting = false;
		}
	}

	/*
	 * #816's press-through: the same enrolment finish, sent again with
	 * X-Confirmed, on the same freshly-minted idToken.
	 */
	async function handleConfirmSignOut(): Promise<void> {
		errors = [];
		isSubmitting = true;
		try {
			const response = await postFinishEnrollment(pendingIdToken, true);
			if (response.ok) {
				await signOut(getFirebaseAuth());
				await landAfterEnrolment();
				return;
			}
			errors = [{ message: await refusalMessage(response) }];
			step = 'setup';
		} finally {
			isSubmitting = false;
		}
	}

	/*
	 * Backing out of #816's warning. Nothing was minted, so she is back on
	 * step two with the same secret and QR code -- and the portal session
	 * she chose to keep is untouched. The just-enrolled TOTP factor is
	 * left in place: Identity Platform, not this screen, owns undoing an
	 * enrolment, and re-submitting the same code finishes what only the
	 * mint was waiting on.
	 */
	function handleCancelSignOut(): void {
		signOutWarning = undefined;
		step = 'setup';
	}

	// Moves focus to #816's warning the moment it replaces the form -- see
	// the sign-in page's own copy of this comment for why.
	function focusOnAppearing(element: HTMLElement) {
		element.focus();
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
	{:else if step === 'confirm-sign-out'}
		<!--
			#816: the warning goes on the button that acts, not on a screen
			of its own -- see the sign-in page's own copy of this pattern.
			Her code is already accepted, so there is nothing to re-render
			here but the consequence and the two ways out of it.
		-->
		<h2 tabindex="-1" {@attach focusOnAppearing}>Before you continue</h2>
		<WarningText message={signOutWarning ?? ''} />
		<Button
			type="button"
			label="Continue and sign out"
			loading={isSubmitting}
			onClick={handleConfirmSignOut}
		/>
		<Button type="button" label="Cancel" variant="secondary" onClick={handleCancelSignOut} />
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
