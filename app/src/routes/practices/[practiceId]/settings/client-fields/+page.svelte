<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import PracticeQuestionList, {
		type ListedQuestion
	} from '#lib/components/organisms/PracticeQuestionList.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { startHref } from '../../clients/new/intake.js';
	import {
		fieldCountWarning,
		loadTemplate,
		saveTemplate,
		unarchiveField,
		type Field
	} from '#lib/clientFieldTemplate.js';
	import { clientFieldsSavedMessage } from './clientFieldPages.js';
	import type { PracticeSession } from '../../+layout.js';

	/*
	 * #2193: GOV.UK's "Add another" list, on the same component as the Plan
	 * Templates (`PracticeQuestionList`). A row per question still asked,
	 * with a Change that opens it on its own page, and one "Add a question"
	 * that asks the kind of answer first. Each question's page saves the
	 * template and comes back here, so this screen has no Save of its own:
	 * what the save did is said before the <h1>, focused on arrival
	 * (`outcome`, #1710), in #2194's words.
	 *
	 * An archived question is not part of the form a Client is asked but a
	 * record of a question this Practice no longer asks (#461), so it is a
	 * list of its own under its own heading, with "Collect again" to restore
	 * it. That one act saves from here, since it only adds the question
	 * back, and says so in the same place.
	 *
	 * Read is every Staff member (ADR-0017), write is Owner or Admin
	 * (#460's RequireOwnerOrAdmin). Server-side enforcement is what actually
	 * holds the line; this is UX only.
	 */
	const session = $derived((page.data as { session: PracticeSession }).session);
	const canChange = $derived(isOwnerOrAdmin(session));
	const practiceId = $derived(page.params.practiceId!);

	let outcome = $state(takeOutcome(page.url));
	let fields = $state<Field[]>([]);
	let isLoaded = $state(false);
	let error = $state('');

	const activeFields = $derived(fields.filter((field) => !field.archived));
	const countWarning = $derived(fieldCountWarning(fields));

	type ArchivedRow = SummaryListRow & { id: string };
	const archivedRows: ArchivedRow[] = $derived(
		fields
			.filter((field) => field.archived)
			.map((field) => {
				const name = field.label || 'Untitled question';
				return { id: field.id, label: name, value: 'No longer asked', changes: canChange ? name : undefined };
			})
	);

	async function load() {
		try {
			const template = await loadTemplate(apiFetchWithSession, practiceId);
			fields = template.fields;
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Client Field Template';
		}
	}

	async function collectAgain(id: string) {
		error = '';
		outcome = undefined;
		try {
			const template = await saveTemplate(apiFetchWithSession, practiceId, unarchiveField(fields, id));
			fields = template.fields;
			outcome = clientFieldsSavedMessage;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to save Client Field Template';
		}
	}

	function changeHref(question: ListedQuestion): string {
		return resolve('/practices/[practiceId]/settings/client-fields/[questionId]', {
			practiceId,
			questionId: question.id
		});
	}

	onMount(() => {
		void load();
	});
</script>

<!--
	#865 aligned this with its two siblings. Three changes, all of them the
	same rule: say what is true of *this* screen. "this Practice", not "your
	Practice", so the settings screens and the Settings hub speak with one
	voice. The list starts empty rather than seeded -- `staffauth/signup.go`
	deliberately seeds no Client fields, which `clientfieldtemplate/
	template.go` calls the designed state for a new Practice. And the
	reassurance is the archive rule, not ADR-0001's snapshot: GLOSSARY.md is
	explicit that a Client's values are read live against the current list,
	so archiving is the only thing standing between a removed field and a
	recorded fact.
-->
{#snippet intro()}
	<Text
		text="The extra questions this Practice asks about every Client, beyond name, contact details and address. Nothing is here to start with, no Client ever sees one, and removing a field archives it, so what was already recorded stays."
	/>
{/snippet}

{#snippet list()}
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
	{#if countWarning}
		<Notice variant="info" message={countWarning} />
	{/if}
	{#if isLoaded}
		<PracticeQuestionList
			questions={activeFields}
			emptyMessage="No extra questions yet."
			changeHref={canChange ? changeHref : undefined}
			addHref={canChange ? resolve('/practices/[practiceId]/settings/client-fields/kind', { practiceId }) : undefined}
		/>
		{#if !canChange}
			<Text text="Ask a Practice Owner or Admin to change the Client Fields." />
		{/if}
	{/if}
{/snippet}

{#snippet archived()}
	<stack-l space="var(--space-3)">
		<Heading level={2} text="Archived questions" />
		<SummaryList rows={archivedRows}>
			{#snippet action(row: ArchivedRow, describedBy: string)}
				<Button label="Collect again" variant="link" {describedBy} onClick={() => collectAgain(row.id)} />
			{/snippet}
		</SummaryList>
	</stack-l>
{/snippet}

{#snippet actions()}
	<!--
		#2194: a Practice with no Client yet is linked on to her first one, as
		the Contract Template is; from the first Client on, the search is the
		door to intake (ADR-0017's amendment for #1516).
	-->
	{#if canChange && session.hasClients === false}
		<Link href={startHref(practiceId, 'client-fields')} label="Next, add your first Client" />
	{/if}
{/snippet}

<FormPage
	title="Client Fields"
	{intro}
	{outcome}
	fieldsets={archivedRows.length > 0 ? [{ content: list }, { content: archived }] : [{ content: list }]}
	{actions}
/>
