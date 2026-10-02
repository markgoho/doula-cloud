<script lang="ts">
	/*
	 * PROTOTYPE -- #1496, screen 4: the Start work form on `FormPage`.
	 * The kind and the due date as built; "Who is the Doula?" (#1596), with
	 * the person at the form selected when she is the only Doula; the
	 * Credits sentence in place of "Credit cost / Balance after" (#1612);
	 * a second action that opens her record and does not say "Cancel"
	 * (#1611). #1611 gives no words for it: "Start work later" is this
	 * prototype's proposal. The Note has no hint about an approver, because
	 * an Owner's request is approved at once (#1512).
	 */
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import { startWorkCredits } from './fixtures.js';
	import { NO_DOULA, SELF, prototype, to } from './model.svelte.js';

	const KIND_NAME = 'engagement-request-kind';
	const DOULA_NAME = 'engagement-request-doula';
	const kindFieldId = `${KIND_NAME}-birth`;
	const doulaFieldId = `${DOULA_NAME}-${SELF}`;
	const dueDateId = 'engagement-request-due-date';
	const noteId = 'engagement-request-note';

	let kind = $state<'' | 'birth' | 'postpartum'>('');
	let dueDate = $state('');
	let note = $state('');
	// #1596: selected only where she is the only Doula at the Practice.
	let doula = $derived(prototype.owner === 'solo' ? SELF : '');
	let errors = $state<FormError[]>([]);

	let filled = prototype.fill;
	$effect(() => {
		if (prototype.fill === filled) return;
		filled = prototype.fill;
		kind = 'birth';
		dueDate = prototype.sample.dueDate;
	});

	const name = $derived(prototype.clientName);
	const submitLabel = $derived(`Start work with ${name}`);
	const isDueDateRequired = $derived(kind === 'birth');
	const errorFor = (id: string) => errors.find((entry) => entry.targetId === id)?.message;
	const doulaOptions = $derived([...prototype.doulas, { value: NO_DOULA, label: 'No Doula yet' }]);

	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (kind === '')
			found.push({ message: 'Select whether this is birth or postpartum work', targetId: kindFieldId });
		if (isDueDateRequired && dueDate.trim() === '')
			found.push({ message: 'Enter the due date', targetId: dueDateId });
		if (doula === '')
			found.push({ message: 'Select who the Doula is, or select "No Doula yet"', targetId: doulaFieldId });
		return found;
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errors = findRefusals();
		if (errors.length > 0) return;
		prototype.startWork({ kind: kind as 'birth' | 'postpartum', dueDate, doula, note });
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet formIntro()}
	<Text text={startWorkCredits(name, prototype.balance)} measure />
{/snippet}

{#snippet requestFields()}
	<RadioGroup
		legend="Kind of work"
		name={KIND_NAME}
		error={errorFor(kindFieldId)}
		options={[
			{ value: 'birth', label: 'Birth' },
			{ value: 'postpartum', label: 'Postpartum' }
		]}
		value={kind}
		onChange={(value: string) => (kind = value as 'birth' | 'postpartum')}
	/>
	<LabeledField
		id={dueDateId}
		label="Due date"
		hint={isDueDateRequired ? undefined : 'Optional for postpartum work'}
		error={errorFor(dueDateId)}
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="date"
				value={dueDate}
				onInput={(value) => (dueDate = value)}
				required={isDueDateRequired}
			/>
		{/snippet}
	</LabeledField>
	<RadioGroup
		legend="Who is the Doula?"
		name={DOULA_NAME}
		error={errorFor(doulaFieldId)}
		options={doulaOptions}
		value={doula}
		onChange={(value: string) => (doula = value)}
	/>
	<LabeledField id={noteId} label="Note (optional)">
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={note} onInput={(value) => (note = value)} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet formActions()}
	<Button type="submit" label={submitLabel} />
	<Link href={to('client')} label="Start work later" variant="secondary" />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template, which stacks its own regions; the `<form>` owns the submit and arranges nothing. -->
<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title={submitLabel}
		intro={formIntro}
		fieldsets={[{ content: requestFields }]}
		errorSummary={errors.length > 0 ? errorSummary : undefined}
		actions={formActions}
	/>
</form>
