<script lang="ts">
	import { createUserWithEmailAndPassword, signInWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { authRefusal, refusalMessage, type FormError } from '#lib/formErrors.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';

	const modeOptions: { value: 'signup' | 'login'; label: string }[] = [
		{ value: 'signup', label: "I'm new here -- create an account" },
		{ value: 'login', label: 'I already have an account -- log in' }
	];

	const inviteToken = page.url.searchParams.get('token') ?? '';

	const emailId = 'portal-accept-invite-email';
	const passwordId = 'portal-accept-invite-password';

	let email = $state('');
	let password = $state('');
	let mode = $state<'signup' | 'login'>('signup');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let picker = $state<Engagement[] | undefined>();

	// Whichever `mode` is chosen decides whether the email is a new
	// identity or an existing one, and whether the password is being set
	// or recalled (#469).
	const emailAutocomplete = $derived(mode === 'signup' ? 'email' : 'username');
	const passwordAutocomplete = $derived(mode === 'signup' ? 'new-password' : 'current-password');

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		picker = undefined;

		const refusals: FormError[] = [];
		if (email.trim() === '')
			refusals.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') {
			refusals.push({ message: 'Enter your password', targetId: passwordId });
		} else if (mode === 'signup' && password.length < 6) {
			// Only when she is creating the account: an existing password is
			// whatever it already is, and refusing a short one here would shut
			// out anyone who set one before the rule existed.
			refusals.push({ message: 'Password must be 6 characters or more', targetId: passwordId });
		}
		if (refusals.length > 0) {
			errors = refusals;
			return;
		}

		isSubmitting = true;
		try {
			const credential =
				mode === 'signup'
					? await createUserWithEmailAndPassword(getFirebaseAuth(), email, password)
					: await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			// A plain, one-off fetch: this token makes one trip and is never
			// carried around the way apiFetchWithSession's cookie is (#150
			// deleted the shared ID-token helper).
			const acceptResponse = await fetch(`${apiBaseURL()}/api/portal/accept-invite`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${idToken}` },
				body: JSON.stringify({ inviteToken })
			});
			if (!acceptResponse.ok) {
				errors = [{ message: await refusalMessage(acceptResponse) }];
				await signOut(getFirebaseAuth());
				return;
			}

			// AcceptInviteHandler already set the session cookie on its own
			// response (#145) -- no separate exchange needed, just drop the
			// JS SDK credential before the session probe reads the cookie.
			await signOut(getFirebaseAuth());

			const sessionResponse = await apiFetchWithSession('/api/portal/session');
			if (!sessionResponse.ok) {
				errors = [{ message: await refusalMessage(sessionResponse) }];
				return;
			}
			const session: PortalSessionInfo = await sessionResponse.json();
			const landing = decidePortalLanding(session);
			if (landing.type === 'redirect') {
				await goto(
					resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: landing.engagementId })
				);
			} else {
				picker = landing.engagements;
			}
		} catch (error_) {
			// Identity Platform's own words name a product and carry a banned
			// adjective; `authRefusal` maps the code onto a message and, where
			// there is one, the control that caused it. It covers both modes
			// this form handles (#467).
			errors = [authRefusal(error_, { emailId, passwordId })];
		} finally {
			isSubmitting = false;
		}
	}
</script>

<ErrorSummary {errors} />

<h1>Accept your portal invite</h1>

{#if !inviteToken}
	<Notice variant="error" message="Missing invite token" />
{:else}
	<!-- `novalidate`: the page refuses the submit, not the browser (#467). -->
	<form onsubmit={handleSubmit} novalidate>
		<LabeledField id={emailId} label="Email" error={errorFor(emailId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
					required
					autocomplete={emailAutocomplete}
				/>
			{/snippet}
		</LabeledField>
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
					minlength={6}
					autocomplete={passwordAutocomplete}
				/>
			{/snippet}
		</LabeledField>
		<RadioGroup
			legend="Account mode"
			name="mode"
			options={modeOptions}
			value={mode}
			onChange={(value) => (mode = value)}
		/>
		<Button type="submit" label="Accept invite" loading={isSubmitting} />
	</form>
{/if}

{#if picker}
	<h2>Choose an Engagement</h2>
	{#if picker.length === 0}
		<p>You don't have an Engagement yet. Ask your Practice to set one up.</p>
	{:else}
		<ul>
			{#each picker as engagement (engagement.engagementId)}
				<li>
					<Link
						href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
						label={engagement.practiceName}
					/>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
