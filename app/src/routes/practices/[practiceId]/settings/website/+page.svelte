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
	import { page } from '$app/state';
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
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

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
			current = await saveWebsite(apiFetchWithSession, page.params.practiceId!, {
				mode: choice === 'hosted' ? 'hosted' : 'own',
				ownUrl,
				serviceDescription,
				cancellationPolicy
			});
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
		options={[
			{ value: 'own', label: 'I have my own website or social profile' },
			{ value: 'hosted', label: 'Publish a page for me' }
		]}
		value={choice}
		onChange={(value: string) => (choice = value as Choice)}
	/>
	{#if fieldErrors.mode}
		<p class="field-error" role="alert">{fieldErrors.mode}</p>
	{/if}
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
	A raw <textarea> with its own label, hint, counter and error, rather
	than LabeledField: the live counter has to be named in
	aria-describedby alongside the hint, and LabeledField builds that list
	from a hint and an error only. #468 is the ticket that turns this into
	a Textarea atom with a character count; until it lands, the wiring is
	spelled out here rather than half-done through a component that cannot
	express it.

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
	remaining: number,
	error: string,
	onInput: (next: string) => void
)}
	<stack-l space="var(--space-1)">
		<label for="{name}-input">{label}</label>
		<p id="{name}-hint" class="hint">{hint}</p>
		<textarea
			id="{name}-input"
			rows="5"
			aria-describedby="{name}-hint {name}-count{error ? ` ${name}-error` : ''}"
			aria-invalid={error ? 'true' : undefined}
			class:invalid={Boolean(error)}
			{value}
			oninput={(event) => onInput(event.currentTarget.value)}
		></textarea>
		<p id="{name}-count" class="count" class:over={remaining < 0} aria-live="polite">
			{remaining < 0
				? `You have ${-remaining} characters too many`
				: `You have ${remaining} characters remaining`}
		</p>
		{#if error}
			<p id="{name}-error" class="field-error" role="alert">{error}</p>
		{/if}
	</stack-l>
{/snippet}

{#snippet hostedFacts()}
	{@render fact(
		'serviceDescription',
		'What your Practice offers',
		'Stripe reads this to understand what Clients are paying for. Say what kind of support you provide and where you work.',
		serviceDescription,
		descriptionRemaining,
		fieldErrors.serviceDescription ?? '',
		(next) => (serviceDescription = next)
	)}
	{@render fact(
		'cancellationPolicy',
		'Your cancellation or refund policy',
		'What happens if a Client cancels, and whether any of what she paid comes back. Stripe requires this on your page.',
		cancellationPolicy,
		policyRemaining,
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
	{#if submitError}
		<Notice variant="error" message={submitError} />
	{/if}
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
	<Notice variant="error" message={loadError} />
{:else if current}
	{#if !isOwner}
		<Notice
			variant="status"
			message="Only a Practice Owner can change this. Ask an Owner if it needs updating."
		/>
	{/if}

	{#if step === 'answers'}
		<form onsubmit={handleContinue}>
			<FormPage
				title="Your website"
				{intro}
				error={errorSummary}
				fieldsets={choice === '' ? [{ content: howFound }] : [{ content: howFound }, { content: choice === 'own' ? ownWebsite : hostedFacts }]}
				{actions}
			/>
		</form>
	{:else if step === 'review'}
		<container-l>
			<center-l max="var(--form-max)" gutters="var(--page-gutter)">
				<stack-l space="var(--space-7)">
					<Heading level={1} variant="page" text="Check your page before you publish it" />
					<Text
						text="This is what Clients and Stripe will see at your page. You can change any of it later."
					/>

					{#if submitError}
						<Notice variant="error" message={submitError} />
					{/if}

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
						<Text
							text="Your page goes live the next time the site is built. Until then Stripe will see the address but not the page."
						/>
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
{/if}

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		label {
			display: block;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		.hint {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		textarea {
			inline-size: 100%;
			min-block-size: calc(var(--space-7) * 4);
			padding: var(--space-3);
			font: inherit;
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
		}

		textarea.invalid {
			border-color: var(--color-error);
		}

		.count {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		/* Over the budget is an error she can still fix by typing, so it
		   reads like one before the submit ever refuses. */
		.count.over {
			color: var(--color-error);
			font-weight: var(--font-weight-medium);
		}

		.field-error {
			margin: 0;
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
