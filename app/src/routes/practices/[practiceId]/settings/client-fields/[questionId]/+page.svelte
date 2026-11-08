<script lang="ts">
	/*
	 * One Client field on its own page (#2193), on the Plan Templates'
	 * question page (`PracticeQuestionPage`). `new` is a field not yet saved,
	 * with the kind the kind page chose (`?kind=`).
	 *
	 * What a Client field does that a Plan Template field does not stays
	 * here. Removing one archives it, behind a confirmation, because a
	 * Client's values are read live against this list (GLOSSARY.md) and an
	 * archived field keeps what was recorded. A saved field keeps its kind
	 * of answer, since the Go BFF refuses a type change once values are read
	 * against it. And a question restating a structural fact is refused
	 * here before the boundary refuses it (`structuralFieldRefusal`).
	 *
	 * The template's write replaces the whole list
	 * (`PUT .../client-field-template`), so this page reads the list as it
	 * stands, puts this one field into it, and sends it back; the BFF records
	 * each real change, before and after, in the Practice's activity. Its
	 * save lands on the list, which says what changed (#2194's words, #1710).
	 * Write is Owner or Admin; anyone else reads who may change it.
	 */
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import {
		archiveField,
		loadTemplate,
		saveTemplate,
		structuralFieldRefusal,
		type Field
	} from '#lib/clientFieldTemplate.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import PracticeQuestionPage from '#lib/components/templates/PracticeQuestionPage.svelte';
	import { clientFieldsArchivedMessage, clientFieldsHref, clientFieldsSavedMessage } from '../clientFieldPages.js';
	import type { PracticeSession } from '../../../+layout.js';

	const session = $derived((page.data as { session: PracticeSession }).session);
	const practiceId = $derived(page.params.practiceId!);
	const listHref = $derived(clientFieldsHref(practiceId));

	async function load() {
		const template = await loadTemplate(apiFetchWithSession, practiceId);
		return template.fields;
	}

	async function send(fields: Field[], message: string) {
		await saveTemplate(apiFetchWithSession, practiceId, fields);
		await gotoWithOutcome(listHref, message);
	}
</script>

{#snippet readOnly()}
	<Notice variant="status" message="Only a Practice Owner or Admin can change the Client Fields." />
{/snippet}

{#if isOwnerOrAdmin(session)}
	<PracticeQuestionPage
		name="client-question"
		caption="Client Fields"
		{listHref}
		questionId={page.params.questionId!}
		kind={page.url.searchParams.get('kind') ?? undefined}
		load={load}
		loadFailure="Failed to load Client Field Template"
		notOnList="This question is not on the Client Fields any more."
		isEditable={(field) => !field.archived}
		locksKind
		refuseLabel={structuralFieldRefusal}
		toField={(draft) => ({ ...draft, order: 0, archived: false })}
		onSave={(fields) => send(fields, clientFieldsSavedMessage)}
		retire={{
			verb: 'Archive',
			consequence:
				"A Client who already answered this keeps that answer, read-only on the Client's record. New Clients will not be asked this question, and you can collect it again later from the archived questions.",
			apply: archiveField,
			onDone: (fields, label) => send(fields, clientFieldsArchivedMessage(label))
		}}
	/>
{:else}
	<FormPage
		title={page.params.questionId === 'new' ? 'Add a question' : 'Change a question'}
		fieldsets={[{ content: readOnly }]}
	/>
{/if}
