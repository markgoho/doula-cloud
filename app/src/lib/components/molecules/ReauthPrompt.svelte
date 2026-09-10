<script lang="ts">
	/*
	 * Step-up re-authentication: prove it is still you, right now.
	 *
	 * Two acts in this app refuse a live session on its own and demand a
	 * *fresh* Identity Platform credential alongside it -- removing your
	 * own second factor, and vouching for a Staff member who has lost hers
	 * (api/internal/staffauth/reauth.go's `RequireRecentAuth`, five-minute
	 * window). Both had to run the same three-part dance: sign in again
	 * with the password, answer the TOTP challenge Identity Platform
	 * raises for an already-enrolled identity, then hand the resulting
	 * `User` to whatever act was waiting on it. #694 is the second
	 * consumer, so the dance is composed once here rather than typed
	 * twice.
	 *
	 * What this does *not* own is the act itself. `onAuthenticated`
	 * receives the freshly authenticated `User` and does whatever the
	 * screen came here to do; a rejection from it lands back on the
	 * password step with its message shown, because the TOTP resolver is
	 * single-use and there is nothing left here to retry against.
	 *
	 * `/mfa/enroll` deliberately does not use this. It re-authenticates
	 * too, but what it does next is open an *enrolment* session against a
	 * factor that does not exist yet, so it never meets the TOTP challenge
	 * this component's second step exists for.
	 */
	import { onMount } from 'svelte';
	import {
		getMultiFactorResolver,
		signInWithEmailAndPassword,
		signOut,
		TotpMultiFactorGenerator,
		type MultiFactorError,
		type MultiFactorResolver,
		type User
	} from 'firebase/auth';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { errorsFromCause, isMultiFactorAuthRequired, passwordReauthRefusal, totpCodeRefusal } from '#lib/formErrors.js';
	import type { FormError, FormSubmission } from '#lib/formSubmission.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';
	import StackedForm from './StackedForm.svelte';
	import TotpCodeField from './TotpCodeField.svelte';

	interface Properties {
		/**
		 * Prefixes the ids of the two controls, so the page's own
		 * `ErrorSummary` links to a real control and two prompts on one
		 * page could never collide (#666).
		 */
		idPrefix: string;
		/**
		The address to re-authenticate as -- her own, always.
		*/
		email: string;
		/**
		Why she is being asked, in the caller's own words.
		*/
		prompt: string;
		/**
		 * The word on the button that *performs the act* -- shown on the
		 * step that actually reaches `onAuthenticated`. The password step
		 * says "Continue" instead, because for anyone who holds a second
		 * factor (which is everyone this prompt is shown to) pressing it
		 * raises the code challenge rather than doing the thing.
		 */
		confirmLabel: string;
		confirmVariant?: 'primary' | 'destructive';
		/**
		 * The page's own submission state. Passed in rather than owned here
		 * so the page renders one `ErrorSummary` over everything that can
		 * refuse it, this prompt included -- ADR-0018's rule that the
		 * summary is a position the route renders, never markup a component
		 * builds.
		 */
		submission: FormSubmission;
		/**
		The act this whole dance was in front of.
		*/
		onAuthenticated: (user: User) => Promise<void> | void;
		/**
		Backing out. Omitted where there is nothing to back out to.
		*/
		onCancel?: () => void;
		/**
		 * True when this prompt is rendered inside a `<form>` the page
		 * already owns -- HTML forbids nesting one form inside another, so
		 * the controls become plain buttons with their own `onClick`,
		 * exactly what the account screen did before this component
		 * existed. Left false, the prompt brings its own `<form>`, which is
		 * what makes Enter submit a password field on a page built around
		 * this one act.
		 */
		insideForm?: boolean;
	}

	let {
		idPrefix,
		email,
		prompt,
		confirmLabel,
		confirmVariant = 'primary',
		submission,
		onAuthenticated,
		onCancel,
		insideForm = false
	}: Properties = $props();

	const passwordId = $derived(`${idPrefix}-password`);
	const codeId = $derived(`${idPrefix}-code`);
	let step = $state<'password' | 'code'>('password');
	let password = $state('');
	let code = $state('');

	const buttonLabel = $derived(step === 'password' ? 'Continue' : confirmLabel);

	/*
	 * The in-progress re-authentication Identity Platform is waiting on,
	 * kept across the two steps without being `$state` -- the code step
	 * reads it once, to resolve, and the markup never does.
	 */
	let resolver: MultiFactorResolver | undefined;

	/*
	 * #167's shared-device rule: a live client-side Identity Platform
	 * sign-in this prompt started must not survive her navigating away
	 * mid-flow. Both handlers already sign out on their own exits; this is
	 * for the exit that skips them, which is leaving the page.
	 */
	onMount(() => signOutOfFirebaseSDK);

	function signOutOfFirebaseSDK(): void {
		void signOut(getFirebaseAuth());
	}

	/*
	 * What both steps end with once Identity Platform has accepted the
	 * credential. The SDK is signed back out either way -- success or
	 * refusal -- because the act is done with the token by the time
	 * `onAuthenticated` settles, and a live SDK session left behind is
	 * exactly what #167 objects to.
	 */
	async function handOff(user: User): Promise<FormError[] | undefined> {
		try {
			await onAuthenticated(user);
			return undefined;
		} catch (error_) {
			/*
			 * Read here rather than left to the step mappers below. Those
			 * translate Identity Platform's own error codes, and a refusal
			 * from the act -- a BFF 4xx naming why it said no -- has no code
			 * for them to match, so it would land on their default and read
			 * as "there is a problem with the service" instead of the one
			 * sentence that actually says what happened.
			 */
			return errorsFromCause(error_);
		} finally {
			await signOut(getFirebaseAuth());
			// The resolver is single-use, so a refusal cannot be retried
			// from the code step -- she starts the step-up over.
			step = 'password';
			password = '';
			code = '';
		}
	}

	async function handlePasswordSubmit(event?: SubmitEvent): Promise<void> {
		event?.preventDefault();
		await submission.run(async () => {
			if (password === '') {
				return [{ message: 'Enter your password', targetId: passwordId }];
			}

			try {
				const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
				return await handOff(credential.user);
			} catch (error_) {
				if (isMultiFactorAuthRequired(error_)) {
					// Expected, and not a refusal: Identity Platform challenges
					// the second factor on every sign-in once one is enrolled,
					// and everyone this prompt is shown to has one.
					resolver = getMultiFactorResolver(getFirebaseAuth(), error_ as MultiFactorError);
					code = '';
					step = 'code';
					return;
				}
				throw error_;
			}
		}, (refusal) => (Array.isArray(refusal) ? refusal : [passwordReauthRefusal(refusal, passwordId)]));
	}

	async function handleCodeSubmit(event?: SubmitEvent): Promise<void> {
		event?.preventDefault();
		await submission.run(async () => {
			if (code.trim() === '') {
				return [{ message: 'Enter the 6-digit code from your authenticator app', targetId: codeId }];
			}

			const assertion = TotpMultiFactorGenerator.assertionForSignIn(resolver!.hints[0].uid, code);
			const credential = await resolver!.resolveSignIn(assertion);
			return await handOff(credential.user);
		}, (refusal) => (Array.isArray(refusal) ? refusal : [totpCodeRefusal(refusal, codeId)]));
	}

	function handleCancel(): void {
		submission.errors = [];
		password = '';
		code = '';
		step = 'password';
		void signOut(getFirebaseAuth());
		onCancel?.();
	}
</script>

{#snippet fields()}
	{#if step === 'password'}
		<Text text={prompt} />
		<LabeledField id={passwordId} label="Password" error={submission.errorFor(passwordId)}>
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
	{:else}
		<!--
			No password field here: Identity Platform already accepted it,
			and asking again would be asking a question that is settled --
			the same shape the sign-in screen's own challenge step uses.
		-->
		<TotpCodeField id={codeId} value={code} onInput={(value) => (code = value)} error={submission.errorFor(codeId)} />
	{/if}
{/snippet}

{#snippet cancelButton()}
	{#if onCancel}
		<Button
			type="button"
			variant="secondary"
			label="Cancel"
			disabled={submission.isSubmitting}
			onClick={handleCancel}
		/>
	{/if}
{/snippet}

{#if insideForm}
	{@render fields()}
	<Button
		type="button"
		variant={confirmVariant}
		label={buttonLabel}
		loading={submission.isSubmitting}
		onClick={() => (step === 'password' ? handlePasswordSubmit() : handleCodeSubmit())}
	/>
	{@render cancelButton()}
{:else}
	<!-- `novalidate` lives on StackedForm: the page refuses the submit, not the browser (#467). -->
	<StackedForm onSubmit={step === 'password' ? handlePasswordSubmit : handleCodeSubmit}>
		{@render fields()}
		<Button
			type="submit"
			variant={confirmVariant}
			label={buttonLabel}
			loading={submission.isSubmitting}
		/>
		{@render cancelButton()}
	</StackedForm>
{/if}
