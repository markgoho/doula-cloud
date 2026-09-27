<script lang="ts">
	/*
	 * PROTOTYPE -- #1502. The feedback form's body, the same in every
	 * variant: charting Q5's kind radio group and one free-text box, Q4's
	 * "We will also send", Q7's destination line. The variants differ only
	 * in where this sits relative to the screen the person came from.
	 */
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import ErrorSummary, { type FormError } from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import { copy, ERROR_KIND, kinds, type Context, type Kind, type Shell } from './fixtures.js';

	interface Properties {
		shell: Shell;
		context: Context;
		onSent: (sent: { kind: Kind; text: string }) => void;
		submitLabel?: string;
	}

	let { shell, context, onSent, submitLabel = 'Send feedback' }: Properties = $props();

	const words = $derived(copy[shell]);
	const idPrefix = $props.id();
	const kindName = `${idPrefix}-kind`;
	const textId = `${idPrefix}-text`;

	let kind = $state<Kind | ''>('');
	let text = $state('');
	let errors = $state<FormError[]>([]);

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (kind === '') {
			errors = [{ message: ERROR_KIND, targetId: radioFieldId(kindName, kinds[0].value) }];
			return;
		}
		errors = [];
		onSent({ kind, text });
	}
</script>

<div class="form">
	{#if errors.length > 0}
		<ErrorSummary {errors} />
	{/if}
	<StackedForm onSubmit={handleSubmit}>
		<RadioGroup
			legend={words.legend}
			name={kindName}
			options={kinds}
			value={kind}
			onChange={(value) => (kind = value)}
			error={errors.length > 0 ? ERROR_KIND : undefined}
		/>
		<LabeledField id={textId} label={words.textLabel} hint={words.textHint}>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea {id} {describedBy} {invalid} value={text} onInput={(value) => (text = value)} rows={5} />
			{/snippet}
		</LabeledField>
		<div class="inset">
			<Text text="We will also send:" step="body-sm" />
			<ul>
				<li>the page you were on: {context.screenTitle}</li>
				<li>your role and Practice: {context.role}, {context.practice}</li>
				<li>the time: {context.time}</li>
				<li>your screen width and browser: {context.width}px, {context.browser}</li>
				<li>the version of Doula Cloud: {context.build}</li>
			</ul>
		</div>
		<Text text={words.destination} />
		<Button type="submit" label={submitLabel} />
	</StackedForm>
</div>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.inset {
		padding: var(--space-3) var(--space-4);
		border-inline-start: var(--border-active) solid var(--color-outline-variant);
	}

	ul {
		margin: var(--space-1) 0 0;
		padding-inline-start: var(--space-5);
		font-size: var(--text-body-sm-size);
		line-height: var(--text-body-sm-leading);
		color: var(--color-on-surface-variant);
	}
</style>
