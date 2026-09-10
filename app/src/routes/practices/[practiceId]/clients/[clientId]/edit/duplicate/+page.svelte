<script lang="ts">
	/*
	 * Gate two's question, on the edit path (ADR-0017's amendment, #814).
	 *
	 * `edit.go` answers 409 with `substitution: false` when the collision
	 * predicate hits and it is not an exact name substitution -- a
	 * possible duplicate, asked rather than blocked. Modelled closely on
	 * `clients/new/duplicate/+page.svelte`'s two-state shape (a bare
	 * question, and a `?match=` reviewing state), with two differences
	 * that shape belongs to intake and not to editing an existing record:
	 *
	 * - **Direction is server-decided.** Intake's matched Client always
	 *   survives. Here `CollisionMatch.wouldSurvive` can go either way --
	 *   an unattached record being edited can itself survive when the
	 *   match it collided with is older -- so every string below names
	 *   whichever side actually survives rather than assuming it.
	 * - **Nothing is ever created.** Choosing "No, a different person"
	 *   on the create path makes a second record; on the edit path the
	 *   record being edited already exists, so the same answer here only
	 *   keeps it as its own record. The wording says so.
	 *
	 * There used to be a third shape here: a page saying no merge was
	 * possible at all, shown whenever the record being edited held an
	 * Engagement, an Engagement Request or a portal account. #813
	 * (ADR-0039) removed it. Two records that both carry history now
	 * merge for real, so "This is her" is offered on every match and
	 * attachment only decides which record survives.
	 *
	 * What the page must still say plainly, because the word "merge"
	 * promises more than the act delivers: the two histories do not
	 * become one. Each Client's history is sealed under her own key and
	 * the log is append-only, so the combined record carries both trails
	 * side by side, each labeled with the record it came from -- and both
	 * Credits stay spent.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { editClient, mergeClient, type CollisionMatch } from '#lib/client.js';
	import { displayName } from '#lib/clientDetail.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
	import { proposedMergeChanges } from '#lib/editMerge.js';

	const JOURNEY = 'Editing a Client';
	const DIFFERENT_PERSON = 'different';
	// Named rather than generated, so the error summary can point an
	// entry at the first radio by id.
	const ANSWER_NAME = 'edit-same-person';

	const practiceId = $derived(page.params.practiceId ?? '');
	const clientId = $derived(page.params.clientId ?? '');

	function editHref(): string {
		return resolve('/practices/[practiceId]/clients/[clientId]/edit', { practiceId, clientId });
	}
	function detailHref(id: string): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', { practiceId, clientId: id });
	}

	const base = $derived(`${editHref()}/duplicate`);
	// One step, not a rail: this page is reached from Client edit rather
	// than from a multi-step sequence, so the journey it names is the
	// single question in front of the reader, not a sequence of them.
	const steps = $derived<JourneyStep[]>([
		{ label: 'Possible duplicate', href: base, status: 'current' }
	]);

	const chosenId = $derived(page.url.searchParams.get('match') ?? '');
	const reviewing = $derived<CollisionMatch | undefined>(
		editMergeDraft.matches.find((match) => match.id === chosenId)
	);
	const changes = $derived(reviewing ? proposedMergeChanges(editMergeDraft.fields, reviewing) : []);

	function survivorName(match: CollisionMatch): string {
		if (match.wouldSurvive) return displayName(match);
		return displayName({ ...editMergeDraft.fields, id: editMergeDraft.clientId });
	}

	let answer = $state('');
	const submission = new FormSubmission();
	let answerError = $state<string | undefined>();

	/*
	 * A reader who reloads this page, or reaches it directly, has nothing
	 * in the draft: it lives in memory and the 409 that filled it is not
	 * repeated by a page load. Back to the edit page, the same shape
	 * intake's own duplicate page uses for the same reason.
	 */
	onMount(() => {
		if (editMergeDraft.matches.length === 0) {
			void goto(editHref());
		}
	});

	function matchDescription(match: CollisionMatch): string {
		const facts = [
			match.dateOfBirth ? `Born ${formatCalendarDay(match.dateOfBirth)}` : 'No date of birth on file',
			match.email || match.phone || 'No contact details on file',
			match.engagements.length === 1 ? '1 Engagement' : `${match.engagements.length} Engagements`
		];
		return facts.join(' · ');
	}

	const options = $derived([
		...editMergeDraft.matches.map((match) => ({
			value: match.id,
			label: displayName(match),
			description: matchDescription(match)
		})),
		{
			value: DIFFERENT_PERSON,
			label: 'No, a different person',
			description: 'Keeps this as its own separate record. Nothing here is merged.'
		}
	]);

	async function saveAsDifferentPerson() {
		await submission.run(async () => {
			const result = await editClient(
				apiFetchWithSession,
				practiceId,
				editMergeDraft.clientId,
				editMergeDraft.fields,
				true
			);
			if (result.conflict) {
				return [{ message: 'The Client record could not be saved.' }];
			}
			const id = editMergeDraft.clientId;
			editMergeDraft.clear();
			await goto(detailHref(id));
		}, orThrownMessage);
	}

	async function saveMerge(match: CollisionMatch) {
		await submission.run(async () => {
			const record = await mergeClient(
				apiFetchWithSession,
				practiceId,
				editMergeDraft.clientId,
				editMergeDraft.fields,
				match.id
			);
			editMergeDraft.clear();
			await goto(detailHref(record.id));
		}, orThrownMessage);
	}

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		if (answer === '') {
			answerError = 'Choose whether this is the same person';
			submission.errors = [{ message: answerError, targetId: `${ANSWER_NAME}-${options[0]!.value}` }];
			return;
		}
		answerError = undefined;
		if (answer === DIFFERENT_PERSON) {
			await saveAsDifferentPerson();
			return;
		}
		const match = editMergeDraft.matches.find((entry) => entry.id === answer);
		// Nothing this merge would change, so there is nothing to confirm --
		// straight to the write, the same shortcut intake's own duplicate
		// page takes. The absorb still happens: this is a write even with
		// zero field changes, since it tombstones one of the two rows.
		if (match && proposedMergeChanges(editMergeDraft.fields, match).length === 0) {
			await saveMerge(match);
			return;
		}
		await goto(`${base}?match=${encodeURIComponent(answer)}`);
	}

	async function handleSaveChanges() {
		if (!reviewing) return;
		await saveMerge(reviewing);
	}
</script>

{#if reviewing}
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref={base}
		question={{ as: 'legend', text: `Save these changes to ${survivorName(reviewing)}?` }}
		hint={`Nothing has been saved yet. No new record is created -- ${survivorName(reviewing)}'s record is kept and updated, and the other is closed as a duplicate.`}
	>
		{#snippet errorSummary()}
			{#if submission.errors.length > 0}
				<ErrorSummary errors={submission.errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<DescriptionList
				items={changes.map((change) => ({
					label: change.label,
					value: `${change.onFile} → ${change.typed}`
				}))}
			/>
		{/snippet}

		{#snippet actions()}
			<Button label="Save changes" loading={submission.isSubmitting} onClick={handleSaveChanges} />
		{/snippet}
	</QuestionPage>
{:else if editMergeDraft.matches.length > 0}
	<form onsubmit={handleContinue} novalidate>
		<QuestionPage
			journey={JOURNEY}
			{steps}
			backHref={editHref()}
			question={{ as: 'legend', text: 'Is this the same person?' }}
			hint="Nothing has been saved yet. What was typed matches a Client this Practice already has."
		>
			{#snippet errorSummary()}
				{#if submission.errors.length > 0}
					<ErrorSummary errors={submission.errors} />
				{/if}
			{/snippet}

			{#snippet content()}
				<stack-l space="var(--space-5)">
					<RadioGroup
						name={ANSWER_NAME}
						{options}
						value={answer}
						onChange={(chosen) => (answer = chosen)}
						error={answerError}
					/>
					<Text
						step="body-sm"
						tone="muted"
						text="Choosing an existing Client combines the two records into one. Engagements, requests and portal access move to the record that is kept, and the record that is absorbed stops appearing anywhere. The one kept is whichever has more history behind it -- never assume which record that is."
					/>
					<Notice
						variant="info"
						message="This cannot be undone, and it does not join the two histories. The combined Client keeps both records' history side by side, each labeled with the record it came from. Any Credits already spent stay spent."
					/>
				</stack-l>
			{/snippet}

			{#snippet actions()}
				<Button type="submit" label="Continue" loading={submission.isSubmitting} />
			{/snippet}
		</QuestionPage>
	</form>
{/if}
