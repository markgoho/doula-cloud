<script lang="ts">
	/*
	 * Sending a piece of Feedback (#1502, #1527): the kind radio group,
	 * the free-text box, the destination line, and the closed disclosure
	 * naming what else the send attaches -- built once here rather than
	 * once per shell, since #1502 Q4's table gives the Staff and Portal
	 * copy the identical shape and differs only in the words.
	 *
	 * Shell-neutral by design (#1527's own hand-off note, since #1528 is
	 * this organism's second consumer): every word is a prop, `onSend`
	 * owns the actual POST the same way `RefundPaymentForm`'s `onConfirm`
	 * does, and the one Staff/Portal fact this form cannot derive on its
	 * own -- the sender's role and Practice -- arrives as `roleAndPractice`
	 * rather than this component reading a session it has no way to know
	 * the shape of.
	 *
	 * Everything else it attaches it reads itself, straight from
	 * `$app/state` (via `#lib/appState.svelte.js`, #597's one seam) and
	 * the browser -- the screen under the drawer, its build stamp, and
	 * the viewport -- because both shells run the same SvelteKit app and
	 * see the same three.
	 */
	import { page } from '#lib/appState.svelte.js';
	import { appBuild } from '#lib/build.js';
	import { browserName, kindOptions, type ClientInput, type Kind } from '#lib/feedback.js';
	import { errorsFromCause, type FormError } from '#lib/formErrors.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Details from '#lib/components/atoms/Details.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';

	interface Properties {
		legend: string;
		errorKind: string;
		textLabel: string;
		textHint: string;
		destination: string;
		detailsSummary: string;
		/**
		 * The one line #1502 Q4's disclosure lists that this form cannot
		 * build on its own -- "your role and Practice" -- since role is a
		 * Staff/Portal-specific concept this shell-neutral organism has no
		 * session to read. Omitted from the list entirely where undefined
		 * (`/account`, which sits outside every Practice), rather than
		 * printed with nothing to say.
		 */
		roleAndPractice?: string;
		submitLabel?: string;
		/**
		 * Posts the send. Throws (a `RefusalError`, matching
		 * `RefundPaymentForm.onConfirm`'s own contract) to report a refusal;
		 * the drafted text stays in the form either way, and the caller
		 * never sees this reject for a missing kind -- that is refused here,
		 * before `onSend` is ever called.
		 */
		onSend: (input: ClientInput) => Promise<void>;
		/**
		 * Called once `onSend` resolves. The caller owns what "sent" means
		 * for its shell -- closing the drawer and announcing a `Notice`, in
		 * both #1527 and #1528 -- so this form only ever reports success and
		 * clears its own draft.
		 */
		onSent: () => void;
	}

	let {
		legend,
		errorKind,
		textLabel,
		textHint,
		destination,
		detailsSummary,
		roleAndPractice,
		submitLabel = 'Send feedback',
		onSend,
		onSent
	}: Properties = $props();

	const idPrefix = $props.id();
	const kindName = `${idPrefix}-kind`;
	const textId = `${idPrefix}-text`;

	let kind = $state<Kind | ''>('');
	let text = $state('');
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);

	// Read at render time (Details is closed until a person opens it), not
	// cached: a person can leave the disclosure open across several
	// keystrokes, and `the time` is truest read fresh rather than frozen
	// at mount.
	function detailsItems(): string[] {
		const items = [`the page you were on: ${page.url.pathname}`];
		if (roleAndPractice) items.push(`your role and Practice: ${roleAndPractice}`);
		items.push(
			`the time: ${new Date().toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' })}`,
			`your screen width and browser: ${window.innerWidth}px, ${browserName(navigator.userAgent)}`,
			`the version of Doula Cloud: ${appBuild()}`
		);
		return items;
	}

	// Set only by the missing-kind refusal below -- an API refusal is only
	// ever reached once `kind` already holds a real value, so the two
	// never disagree about which one produced `errors`.
	const kindFieldError = $derived(kind === '' && errors.length > 0 ? errorKind : undefined);
	const textFieldError = $derived(errors.find((error) => error.targetId === textId)?.message);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (kind === '') {
			errors = [{ message: errorKind, targetId: radioFieldId(kindName, kindOptions[0].value) }];
			return;
		}

		errors = [];
		isSending = true;
		try {
			await onSend({
				kind,
				text,
				page: { url: `${page.url.pathname}${page.url.search}`, route: { id: page.route.id ?? '' } },
				appBuild: appBuild(),
				screenWidth: window.innerWidth
			});
			kind = '';
			text = '';
			onSent();
		} catch (error) {
			errors = errorsFromCause(error, { text: textId });
		} finally {
			isSending = false;
		}
	}
</script>

<div class="form">
	{#if errors.length > 0}
		<ErrorSummary {errors} />
	{/if}
	<StackedForm onSubmit={handleSubmit}>
		<RadioGroup
			{legend}
			name={kindName}
			options={kindOptions}
			value={kind}
			onChange={(value) => (kind = value)}
			error={kindFieldError}
		/>
		<LabeledField id={textId} label={textLabel} hint={textHint} error={textFieldError}>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea {id} {describedBy} {invalid} value={text} onInput={(value) => (text = value)} rows={5} />
			{/snippet}
		</LabeledField>
		<Text text={destination} />
		<!-- GOV.UK Details: disclosed, not called out (#1502 founder note).
		     Nothing is sent that this list does not name (ADR-0046), but a
		     person does not have to read it to send. -->
		<Details summary={detailsSummary}>
			<ul>
				{#each detailsItems() as item, index (index)}
					<li>{item}</li>
				{/each}
			</ul>
		</Details>
		<Button type="submit" label={submitLabel} loading={isSending} />
	</StackedForm>
</div>

<style>
	@layer components {
		.form {
			display: flex;
			flex-direction: column;
			gap: var(--space-4);
		}

		ul {
			margin: 0;
			padding-inline-start: var(--space-5);
			font-size: var(--text-body-sm-size);
			line-height: var(--text-body-sm-leading);
			color: var(--color-on-surface-variant);
		}
	}
</style>
