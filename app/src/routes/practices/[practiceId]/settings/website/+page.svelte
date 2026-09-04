<script lang="ts">
	/*
	 * Where a Practice answers Stripe's website question (#440).
	 *
	 * Stripe's hosted onboarding demands a website from every connected
	 * account. #421 walked what happens when it does not get one: the field
	 * accepts empty, she completes every remaining step, submits, and comes
	 * back to us "done" with charges_enabled false and nothing on screen
	 * saying why. So she is asked here, before she is ever sent to Stripe,
	 * and #442 makes the payments screen refuse to open the flow until this
	 * page has an answer.
	 *
	 * Two answers, and both are real. The fourteen-doula agency almost
	 * certainly has a website already and says so in one click; the solo
	 * doula who has only an Instagram profile gets a page published for her
	 * at doula.cloud/p/<slug> (#441) rather than being told to go and build
	 * a website first.
	 *
	 * The hosted page asks her for exactly two things, because they are the
	 * only two nobody else can supply -- what she offers, and what happens
	 * when a Client cancels. Everything else the page carries is assembled
	 * from what she has already told us. A blank box for the rest would let
	 * her publish something Stripe rejects.
	 *
	 * Publishing is a second step on purpose: this is GOV.UK's
	 * check-your-answers shape, and what she is about to do is put words
	 * under our domain that Stripe reviews and Clients read.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		loadWebsite,
		saveWebsite,
		WebsiteValidationError,
		MAX_FACT_LENGTH,
		type PracticeWebsite
	} from '#lib/website.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/formErrors.js';

	type Choice = '' | 'own' | 'hosted';

	let current = $state<PracticeWebsite | undefined>();
	let practiceName = $state('');
	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let loadError = $state('');

	/* 'answers' collects, 'review' shows the assembled page back to her,
	   'saved' is the state she lands in after a successful write and the
	   one she meets on every later visit. */
	let step = $state<'answers' | 'review' | 'saved'>('answers');
	let choice = $state<Choice>('');
	let ownUrl = $state('');
	let serviceDescription = $state('');
	let cancellationPolicy = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let submitError = $state('');
	let isSubmitting = $state(false);

	let descriptionRemaining = $derived(MAX_FACT_LENGTH - [...serviceDescription].length);
	let policyRemaining = $derived(MAX_FACT_LENGTH - [...cancellationPolicy].length);

	onMount(async () => {
		try {
			current = await loadWebsite(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load your website settings';
			return;
		}

		/* Whatever she has already said is what the form starts from, so
		   coming back to change one word does not mean retyping the rest. */
		ownUrl = current.ownUrl;
		serviceDescription = current.serviceDescription;
		cancellationPolicy = current.cancellationPolicy;
		if (current.mode !== 'undeclared') {
			choice = current.mode;
			step = 'saved';
		}

		/* One call for the Practice's name -- which the hosted page shows as
		   the business name -- and for the caller's roles. The button's
		   enabled state mirrors the payments screen's Owner gating;
		   RequireOwner on the endpoint is what actually enforces it. */
		const sessionResponse = await apiFetchWithSession(
			`/api/practices/${page.params.practiceId}/session`
		);
		if (sessionResponse.ok) {
			const body: { practiceName: string; roles: string[] } = await sessionResponse.json();
			practiceName = body.practiceName;
			roles = body.roles;
		}
	});

	/*
	 * The ids the error summary links to (#467). The radio group's are
	 * built by `RadioGroup` from its `name` and each option's value, so the
	 * group's target is its first option -- GOV.UK sends a reader to the
	 * first control of a refused group, a <fieldset> not being focusable.
	 * The two textareas name their own, passed down to `LabeledField` so
	 * the anchor lands on the control rather than on a generated id.
	 */
	const MODE_NAME = 'website-mode';
	const modeFieldId = `${MODE_NAME}-own`;
	const ownUrlId = 'website-own-url';
	const targetByField: Record<string, string> = {
		mode: modeFieldId,
		ownUrl: ownUrlId,
		serviceDescription: 'serviceDescription-input',
		cancellationPolicy: 'cancellationPolicy-input'
	};
	// Field order, not object order: the summary is read top to bottom and
	// its entries have to match the order of the questions below it.
	const FIELD_ORDER = ['mode', 'ownUrl', 'serviceDescription', 'cancellationPolicy'];

	/*
	 * One array, built from the field errors that already existed here plus
	 * whatever the save refused for. The field keeps rendering its own
	 * message from the same `fieldErrors` record, so the two wordings are
	 * one string shown twice rather than two strings kept in step.
	 */
	const summaryErrors = $derived<FormError[]>([
		...FIELD_ORDER.filter((field) => fieldErrors[field]).map((field) => ({
			message: fieldErrors[field]!,
			targetId: targetByField[field]
		})),
		// No target: a refused save is about the submission, not about a box
		// on the page.
		...(submitError ? [{ message: submitError }] : [])
	]);

	function localErrors(): Record<string, string> {
		const errors: Record<string, string> = {};
		if (choice === '') {
			errors.mode = 'Choose how Clients will find you online';
			return errors;
		}
		if (choice === 'own') {
			if (ownUrl.trim() === '') {
				errors.ownUrl = 'Enter the web address of your website or social profile';
			}
			return errors;
		}
		if (serviceDescription.trim() === '') {
			errors.serviceDescription = 'Enter a description of what your Practice offers';
		} else if (descriptionRemaining < 0) {
			errors.serviceDescription = `Shorten this to ${MAX_FACT_LENGTH} characters or fewer`;
		}
		if (cancellationPolicy.trim() === '') {
			errors.cancellationPolicy = 'Enter your cancellation or refund policy';
		} else if (policyRemaining < 0) {
			errors.cancellationPolicy = `Shorten this to ${MAX_FACT_LENGTH} characters or fewer`;
		}
		return errors;
	}

	/* Declaring her own site is one step -- nothing is published under our
	   domain, so there is nothing to show her first. Publishing a page is
	   two, and this is where the first ends. */
	function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		submitError = '';
		const errors = localErrors();
		fieldErrors = errors;
		if (Object.keys(errors).length > 0) {
			return;
		}
		if (choice === 'hosted') {
			step = 'review';
			return;
		}
		void save();
	}

	async function save() {
		submitError = '';
		isSubmitting = true;
		try {
			/*
			 * Only the fields the chosen mode is about. The server keeps what
			 * the other mode wrote (00045's upsert COALESCEs), so sending them
			 * back buys nothing -- and it costs: a half-written page left over
			 * from a change of mind would be validated against the budget it
			 * has not met, and the 400 would name a box this screen is no
			 * longer showing. A button that silently does nothing is the one
			 * outcome worth designing out.
			 */
			current = await saveWebsite(
				apiFetchWithSession,
				page.params.practiceId!,
				choice === 'hosted'
					? { mode: 'hosted', serviceDescription, cancellationPolicy }
					: { mode: 'own', ownUrl }
			);
			fieldErrors = {};
			step = 'saved';
		} catch (error_) {
			if (error_ instanceof WebsiteValidationError) {
				/* The server refused a field. Back to the questions, with its
				   message beside the input it is about -- a review screen
				   showing an error about a box that is not on it would be a
				   dead end. */
				fieldErrors = error_.fieldErrors;
				step = 'answers';
				return;
			}
			submitError = error_ instanceof Error ? error_.message : 'Failed to save your website';
		} finally {
			isSubmitting = false;
		}
	}

	function reportedOn(iso: string): string {
		if (iso === '') {
			return '';
		}
		return new Date(iso).toLocaleDateString('en-US', {
			day: 'numeric',
			month: 'long',
			year: 'numeric'
		});
	}
</script>

{#snippet howFound()}
	<RadioGroup
		legend="How will Clients and Stripe find you online?"
		name={MODE_NAME}
		options={[
			{ value: 'own', label: 'I have my own website or social profile' },
			{ value: 'hosted', label: 'Publish a page for me' }
		]}
		value={choice}
		onChange={(value: string) => (choice = value as Choice)}
		error={fieldErrors.mode}
	/>
{/snippet}

{#snippet ownWebsite()}
	<!--
		type="text", not type="url". The browser refuses to submit a form
		holding an invalid type="url" field, and "facebook.com/your-practice"
		is invalid to it -- which is exactly the address a solo doula types
		and exactly the one the server normalizes rather than rejects. A
		native refusal here would block the answer we want, in words we did
		not write. inputmode still asks a phone for the URL keyboard.
	-->
	<LabeledField
		id={ownUrlId}
		label="The web address of your website or social profile"
		hint="A Facebook page or an Instagram profile counts. For example, https://facebook.com/your-practice"
		error={fieldErrors.ownUrl}
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				inputmode="url"
				value={ownUrl}
				onInput={(value) => (ownUrl = value)}
			/>
		{/snippet}
	</LabeledField>
{/snippet}

<!--
	`LabeledField` + `Textarea`, since #468 made the count part of the atom.
	The two facts are the only fields in the application with a
	server-enforced maximum -- `website.MaxFactLength`, refused by the
	handler and again by 00045's CHECK -- which is why they are the only
	two that carry a counter at all.

	The ids stay caller-supplied, because the error summary above links to
	them by name.

	No maxlength. GOV.UK's research is that a hard cap silently truncates
	pasted text, so the count is allowed to go negative, turns red, and the
	submit is refused -- here and again in the handler, which is the
	boundary that can enforce it.
-->
{#snippet fact(
	name: string,
	label: string,
	hint: string,
	value: string,
	error: string,
	onInput: (next: string) => void
)}
	<LabeledField id="{name}-input" {label} {hint} error={error || undefined}>
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} {value} {onInput} rows={5} maxLength={MAX_FACT_LENGTH} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet hostedFacts()}
	{@render fact(
		'serviceDescription',
		'What your Practice offers',
		'Stripe reads this to understand what Clients are paying for. Say what kind of support you provide and where you work.',
		serviceDescription,
		fieldErrors.serviceDescription ?? '',
		(next) => (serviceDescription = next)
	)}
	{@render fact(
		'cancellationPolicy',
		'Your cancellation or refund policy',
		'What happens if a Client cancels, and whether any of what the Client paid comes back. Stripe requires this on your page.',
		cancellationPolicy,
		fieldErrors.cancellationPolicy ?? '',
		(next) => (cancellationPolicy = next)
	)}
{/snippet}

{#snippet intro()}
	<Text
		text="Stripe will not let you take Client payments until you have a website it can look at. Tell us where yours is, or let us publish one for you."
	/>
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary errors={summaryErrors} />
{/snippet}

{#snippet actions()}
	<Button
		type="submit"
		label={choice === 'hosted' ? 'Continue' : 'Save'}
		loading={isSubmitting}
		disabled={!isOwner}
	/>
	{#if step === 'answers' && current && current.mode !== 'undeclared'}
		<Button variant="secondary" label="Cancel" onClick={() => (step = 'saved')} />
	{/if}
{/snippet}

{#if loadError}
	<!-- Same frame the other three steps below build by hand -- #480. -->
	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<stack-l space="var(--space-7)">
				<Heading level={1} variant="page" text="Your website" />
				<Notice variant="error" message={loadError} />
			</stack-l>
		</center-l>
	</container-l>
{:else if current}
	{#if !isOwner}
		<Notice
			variant="status"
			message="Only a Practice Owner can change this. Ask an Owner if it needs updating."
		/>
	{/if}

	{#if step === 'answers'}
		<!-- `novalidate`: this page refuses the submit and says so once, at
		     the top, rather than the browser stopping at the first empty
		     field (#467). -->
		<form onsubmit={handleContinue} novalidate>
			<FormPage
				title="Your website"
				{intro}
				errorSummary={summaryErrors.length > 0 ? errorSummary : undefined}
				fieldsets={choice === '' ? [{ content: howFound }] : [{ content: howFound }, { content: choice === 'own' ? ownWebsite : hostedFacts }]}
				{actions}
			/>
		</form>
	{:else if step === 'review'}
		<container-l>
			<center-l max="var(--form-max)" gutters="var(--page-gutter)">
				<stack-l space="var(--space-7)">
					<!-- Above the <h1>, GOV.UK's position and the one `FormPage`
					     takes on the step before this (#467). -->
					<ErrorSummary errors={summaryErrors} />

					<Heading level={1} variant="page" text="Check your page before you publish it" />
					<Text
						text="This is what Clients and Stripe will see at your page. You can change any of it later."
					/>

					<DescriptionList
						items={[
							{ label: 'Business name', value: practiceName },
							{ label: 'What you offer', value: serviceDescription },
							{ label: 'Cancellation policy', value: cancellationPolicy }
						]}
					/>

					<Text
						text="Your page will also show how to reach you and a privacy statement. Those come from what you have already told us, so there is nothing more to write."
					/>

					<cluster-l space="var(--space-3)" align="center">
						<Button label="Publish page" loading={isSubmitting} onClick={save} disabled={!isOwner} />
						<Button variant="secondary" label="Back" onClick={() => (step = 'answers')} />
					</cluster-l>
				</stack-l>
			</center-l>
		</container-l>
	{:else}
		<container-l>
			<center-l max="var(--form-max)" gutters="var(--page-gutter)">
				<stack-l space="var(--space-7)">
					<Heading level={1} variant="page" text="Your website" />

					{#if current.mode === 'own'}
						<DescriptionList
							items={[
								{ label: 'What Stripe will be told', value: current.ownUrl },
								{ label: 'A page published here', value: 'No' }
							]}
						/>
					{:else}
						<DescriptionList
							items={[
								{ label: 'Business name', value: practiceName },
								{ label: 'What you offer', value: current.serviceDescription },
								{ label: 'Cancellation policy', value: current.cancellationPolicy }
							]}
						/>
						<!-- Whether the page is actually there (#443). Three states,
						     and the middle one is the point: a page nothing has
						     confirmed reads as "not confirmed yet", never as a
						     success, because the build can fail and report nothing
						     at all. -->
						{#if current.pageState === 'live'}
							<Text text={`Your page is live at ${current.pageUrl}`} />
						{:else if current.pageState === 'failed'}
							<Notice
								variant="error"
								message={`We could not load your page: ${current.pageCheckDetail}. Choose Change below and save again. If it keeps failing, tell us — Stripe will not let you take payments while your address shows nothing.`}
							/>
						{:else}
							<Text
								text="Your page is being published. This usually takes a few minutes. Until we have loaded it ourselves we will not say it is live."
							/>
						{/if}
					{/if}

					<!-- How this came to say what it says: who last wrote it, and
					     when. A Practice with more than one Owner has more than one
					     person who could have. -->
					{#if current.updatedAt}
						<Text
							step="meta"
							tone="muted"
							text={`Last changed by ${current.updatedBy} on ${reportedOn(current.updatedAt)}`}
						/>
					{/if}

					{#if isOwner}
						<cluster-l space="var(--space-3)" align="center">
							<Button
								variant="secondary"
								label="Change"
								onClick={() => {
									fieldErrors = {};
									submitError = '';
									step = 'answers';
								}}
							/>
						</cluster-l>
					{/if}
				</stack-l>
			</center-l>
		</container-l>
	{/if}
{:else}
	<!-- The loading gap between mount and the first response: previously
	     nothing rendered here at all, not even outside the frame. -->
	<container-l>
		<center-l max="var(--form-max)" gutters="var(--page-gutter)">
			<stack-l space="var(--space-7)">
				<Heading level={1} variant="page" text="Your website" />
				<Skeleton variant="text" lines={4} label="Loading your website settings" />
			</stack-l>
		</center-l>
	</container-l>
{/if}

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}
	}
</style>
