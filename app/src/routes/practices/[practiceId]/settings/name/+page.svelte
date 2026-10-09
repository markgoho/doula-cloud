<script lang="ts">
	/*
	 * Where an Owner states her Practice's name again (#1540). The name she
	 * typed at signup is otherwise permanent: a misspelling stays, and a
	 * doula who started under her own name cannot take a business name.
	 *
	 * The screen opens on the name already on the Practice session, so it
	 * reads nothing of its own. Saving re-runs the Practice layout's load
	 * (`invalidateAll`), which is what moves the name in the header and on
	 * every other screen under this Practice.
	 *
	 * Owner only (the BFF is the authority). The hub shows the link to an
	 * Owner alone, but a URL typed by anyone else lands on the name as a
	 * fact, not on a Save button that exists only to be refused.
	 */
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { invalidateAll } from '$app/navigation';
	import { savePracticeName } from '#lib/practiceName.js';
	import { isOwner } from '#lib/roles.js';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { FormSubmission, type FormError } from '#lib/formSubmission.svelte.js';
	import type { PracticeSession } from '../../+layout.js';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	const nameId = 'practice-name';

	const session = $derived((page.data as { session: PracticeSession }).session);
	let canChangeName = $derived(isOwner(session));

	// Local to the screen so typing does not touch the session until the
	// BFF has accepted it. Seeded once from the session; a save replaces it
	// with the name the BFF stored (trimmed).
	let name = $state((page.data as { session: PracticeSession }).session.practiceName);
	// The name the last save stored, or empty. A status Notice (not plain
	// text) so a screen reader hears it, and it names the result and what
	// follows. Cleared when the field is edited again, so it never
	// describes a state that no longer holds.
	let savedName = $state('');
	const submission = new FormSubmission();

	// A refusal keyed to the `name` field lands on the control and in the
	// summary's link to it (docs/api-design.md section 7 rule 4).
	function mapRefusal(refusal: unknown): FormError[] {
		return Array.isArray(refusal) ? refusal : errorsFromCause(refusal, { name: nameId });
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		savedName = '';
		await submission.run(async () => {
			const saved = await savePracticeName(apiFetchWithSession, page.params.practiceId!, name);
			name = saved.name;
			savedName = saved.name;
			await invalidateAll();
		}, mapRefusal);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet readOnlyFields()}
	<DescriptionList items={[{ label: 'Practice name', value: session.practiceName }]} />
	<Notice variant="status" message="Only a Practice Owner can change the Practice name." />
{/snippet}

{#snippet fields()}
	{#if savedName !== ''}
		<Notice
			variant="status"
			message={`Practice name changed to ${savedName}. Your Clients see it from now on.`}
		/>
	{/if}
	<LabeledField id={nameId} label="Practice name" error={submission.errorFor(nameId)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				value={name}
				onInput={(value) => {
					name = value;
					savedName = '';
				}}
				autocomplete="organization"
			/>
		{/snippet}
	</LabeledField>
	<!--
		GOV.UK's rule for a consequence a person can still act on: say it
		before the press. The name reaches the places a Client meets it, and
		stays where a Client has already seen the old one or Stripe holds
		its own copy.
	-->
	<Text
		text="Your Clients see this name on their portal, on your Practice Page, in the email they get from you, and on every Contract you have not sent yet."
	/>
	<Text
		text="A Contract that is already sent or signed keeps the name it was sent with. Your Practice Page keeps its web address."
	/>
	<Text
		text="The name on your Stripe statement does not change with this. You change it in your Stripe dashboard."
	/>
{/snippet}

{#snippet actions()}
	{#if canChangeName}
		<Button type="submit" label="Save" loading={submission.isSubmitting} />
	{/if}
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `FormPage` stacks each fieldset's content itself, so the `<form>` here owns the submit and arranges nothing; a `StackedForm` would put a second, empty stack around one child. -->
<form onsubmit={save} novalidate>
	<FormPage
		title="Practice name"
		fieldsets={[{ content: canChangeName ? fields : readOnlyFields }]}
		{actions}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	/>
</form>
