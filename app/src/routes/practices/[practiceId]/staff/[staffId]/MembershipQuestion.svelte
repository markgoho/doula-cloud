<script lang="ts">
	/*
	 * One half of a Membership, asked as its own question page (#2041): the
	 * roles page and the employment type page are this, each with its own
	 * question and controls. A Change on the Staff member's page opens one,
	 * the save returns to that page, and the page says what changed in a
	 * status Notice (`outcome.ts`) -- GOV.UK's Summary list row action and
	 * Question pages, the record-page row of `govuk-alignment.md`.
	 *
	 * The PATCH takes both halves together (RA-G2, #261), so each page sends
	 * the half it asks about as answered and the other half as it stands.
	 * The BFF records one Membership event per half that actually moved, so
	 * a roles change never writes an employment type entry (membership.go).
	 *
	 * Owner-only, matching the endpoint's own guard: an Admin who types the
	 * address meets a notice saying who can, not a 403 after she answers --
	 * the shape `mfa-recovery` already uses. Drawing, never a gate.
	 */
	import { onMount, type Snippet } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { updateMembership, type StaffSummary } from '#lib/staff.js';
	import { isOwner as checkIsOwner, type EmploymentType } from '#lib/roles.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { FormSubmission, orThrownMessage, type FormError } from '#lib/formSubmission.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import type { PracticeSession } from '../../+layout.js';
	import { loadMember, memberHref } from './member.js';

	interface Membership {
		roles: string[];
		employmentType: EmploymentType;
	}

	interface Properties {
		/**
		 * What the page changes, in a running sentence: "roles",
		 * "employment type".
		 */
		subject: string;
		/**
		 * The question, which names the person (#463's rule: no pronoun).
		 */
		question: (name: string) => string;
		/**
		 * Called once with the Membership as it stands, so the page opens on
		 * the stored answer.
		 */
		start: (member: StaffSummary) => void;
		/**
		 * The refusals a submit meets before it is sent, or none.
		 */
		validate: () => FormError[];
		/**
		 * The whole Membership to save: the answered half and the other half
		 * as it stands.
		 */
		next: (member: StaffSummary) => Membership;
		/**
		 * What the Staff member's page says once the save has landed.
		 */
		outcome: (member: StaffSummary, saved: Membership) => string;
		controls: Snippet<[FormSubmission]>;
	}

	let { subject, question, start, validate, next, outcome, controls }: Properties = $props();

	const practiceId = $derived(page.params.practiceId!);
	const staffId = $derived(page.params.staffId!);
	const isOwner = $derived(checkIsOwner((page.data as { session: PracticeSession }).session));
	const backHref = $derived(memberHref(practiceId, staffId));

	let member = $state<StaffSummary | undefined>();
	let isRefused = $state(false);
	let loadError = $state('');
	const submission = new FormSubmission();

	onMount(async () => {
		if (!isOwner) {
			isRefused = true;
			return;
		}
		try {
			const loaded = await loadMember(apiFetchWithSession, practiceId, staffId);
			start(loaded);
			member = loaded;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load this Staff member';
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const refusals = validate();
			if (refusals.length > 0) return refusals;
			const saved = next(member!);
			await updateMembership(apiFetchWithSession, practiceId, staffId, saved.roles, saved.employmentType);
			await gotoWithOutcome(backHref, outcome(member!, saved));
		}, orThrownMessage);
	}
</script>

<!-- Passed only while there is a refusal, never declared as a child of
     the Template, so a first visit is not titled "Error: " (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet refusal()}
	<Notice variant="info" message={`Only a Practice owner can change ${subject}. Ask an owner of this Practice to do it.`} />
{/snippet}

{#snippet back()}
	<Link href={backHref} label="Back" />
{/snippet}

{#if member}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` stacks its own regions, so the `<form>` here exists to put the submit inside the form that owns the inputs, not to arrange anything. -->
	<form onsubmit={handleSubmit} novalidate>
		<QuestionPage
			journey={`Changing ${member.name}'s ${subject}`}
			steps={[]}
			{backHref}
			question={{ as: 'legend', text: question(member.name) }}
			errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content()}
				{@render controls(submission)}
			{/snippet}

			{#snippet actions()}
				<Button type="submit" label="Save" loading={submission.isSubmitting} />
			{/snippet}
		</QuestionPage>
	</form>
{:else}
	<FormPage
		title={`Change ${subject}`}
		fieldsets={isRefused ? [{ content: refusal }] : []}
		actions={back}
		loading={isRefused || loadError ? undefined : 'Loading this Staff member'}
		loadError={loadError || undefined}
	/>
{/if}
