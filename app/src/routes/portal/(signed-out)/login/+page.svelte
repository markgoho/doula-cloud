<script lang="ts">
	import { signInWithEmailAndPassword, signOut } from 'firebase/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import { authRefusal, refusalMessage, type FormError } from '#lib/formErrors.js';

	const emailId = 'portal-login-email';
	const passwordId = 'portal-login-password';

	let email = $state('');
	let password = $state('');
	let errors = $state<FormError[]>([]);
	let isSubmitting = $state(false);
	let picker = $state<Engagement[] | undefined>();

	// The Staff login's mechanism, deliberately identical (#467): the two
	// screens ask the same two questions and must refuse them the same way.
	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	function findEmptyFields(): FormError[] {
		const found: FormError[] = [];
		if (email.trim() === '') found.push({ message: 'Enter your email address', targetId: emailId });
		if (password === '') found.push({ message: 'Enter your password', targetId: passwordId });
		return found;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		picker = undefined;

		const empty = findEmptyFields();
		if (empty.length > 0) {
			errors = empty;
			return;
		}

		isSubmitting = true;
		try {
			const credential = await signInWithEmailAndPassword(getFirebaseAuth(), email, password);
			const idToken = await credential.user.getIdToken();

			// Exchange the Identity Platform ID token for the session cookie
			// before signing out of the JS SDK -- the portal session probe
			// right below this authenticates by cookie, so the exchange must
			// land first (#150). A plain, one-off fetch: this token makes one
			// trip and is never carried around the way apiFetchWithSession's
			// cookie is (#150 deleted the shared ID-token helper).
			const exchangeResponse = await fetch(`${apiBaseURL()}/api/session`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${idToken}` }
			});
			if (!exchangeResponse.ok) {
				errors = [{ message: await refusalMessage(exchangeResponse) }];
				await signOut(getFirebaseAuth());
				return;
			}
			await signOut(getFirebaseAuth());

			const response = await apiFetchWithSession('/api/portal/session');
			if (!response.ok) {
				errors = [{ message: await refusalMessage(response) }];
				return;
			}

			const session: PortalSessionInfo = await response.json();
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
			// adjective; `authRefusal` maps the code onto a message and the
			// control that caused it (#467).
			errors = [authRefusal(error_, { emailId, passwordId })];
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet content()}
	<!-- `novalidate`: this page refuses the submit, not the browser. See the
	     Staff login for the argument. -->
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
					autocomplete="username"
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
					autocomplete="current-password"
				/>
			{/snippet}
		</LabeledField>
		<Button type="submit" label="Log in" loading={isSubmitting} />
	</form>

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
{/snippet}

<EntryPage title="Log in" errorSummary={errors.length > 0 ? errorSummary : undefined} {content} />
