<script lang="ts">
	/*
	 * The first page of setting up a Practice (#2049): her name, and the
	 * email address and password she signs in with. Every "Set up a
	 * Practice" door opens here. See `signupJourney.svelte.ts` for the
	 * journey and where its draft lives.
	 *
	 * #1536 (decided on #1493): her name comes first, because an empty
	 * Practice name, two pages on, takes it.
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StaffNameFields from '#lib/components/molecules/StaffNameFields.svelte';
	import { PASSWORD_HINT, PASSWORD_RULES, preloadCommonPasswords } from '#lib/passwordRule.js';
	import SignupQuestion from './SignupQuestion.svelte';
	import { FIELD_IDS, signupDraft, youRefusals } from './signupJourney.svelte.js';

	// #1539: the list loads while she types, not when she continues.
	preloadCommonPasswords();
</script>

<SignupQuestion
	stepId="you"
	question={{ as: 'legend', text: 'About you' }}
	validate={() => youRefusals(signupDraft.answers, signupDraft.password)}
>
	{#snippet controls({ errorFor })}
		<stack-l space="var(--space-5)">
			<StaffNameFields
				bind:firstName={
					() => signupDraft.answers.firstName, (value) => signupDraft.update({ firstName: value })
				}
				bind:lastName={
					() => signupDraft.answers.lastName, (value) => signupDraft.update({ lastName: value })
				}
				firstId={FIELD_IDS.firstName}
				lastId={FIELD_IDS.lastName}
				firstError={errorFor(FIELD_IDS.firstName)}
				lastError={errorFor(FIELD_IDS.lastName)}
			/>
			<LabeledField id={FIELD_IDS.email} label="Email" error={errorFor(FIELD_IDS.email)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="email"
						value={signupDraft.answers.email}
						onInput={(value) => signupDraft.update({ email: value })}
						required
						autocomplete="email"
					/>
				{/snippet}
			</LabeledField>
			<!-- ADR-0026's 15-character rule, stated before she types (#1538),
			     and told to a password manager so a generated one passes it
			     (#1813). -->
			<LabeledField
				id={FIELD_IDS.password}
				label="Password"
				hint={PASSWORD_HINT}
				error={errorFor(FIELD_IDS.password)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="password"
						value={signupDraft.password}
						onInput={(value) => (signupDraft.password = value)}
						required
						minlength={6}
						autocomplete="new-password"
						passwordrules={PASSWORD_RULES}
					/>
				{/snippet}
			</LabeledField>
		</stack-l>
	{/snippet}
</SignupQuestion>
