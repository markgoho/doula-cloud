<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { replaceState } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { MAY_CHANGE } from '#lib/settingsSummary.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PracticeQuestionList, {
		type ListedQuestion
	} from '#lib/components/organisms/PracticeQuestionList.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { loadTemplate, type Field } from '#lib/planTemplate.js';
	import { PLAN_TYPES, planTypeFromQuery, type PlanType } from './planTemplatePages.js';
	import type { PracticeSession } from '../../+layout.js';

	/*
	 * #2193 (MO-G12): GOV.UK's "Add another" list. The questions of the plan
	 * type shown are a Summary list, a row each with a Change that opens the
	 * question on its own page, and one "Add a question" that asks the kind
	 * of answer first. Each question's page saves the template and comes
	 * back here, so this screen has no Save of its own: what the save did is
	 * said here, before the <h1>, focused on arrival (`outcome`, #1710).
	 *
	 * Which plan type is shown is in the address (`?plan=birth_plan`), so a
	 * question's Back and its save land on the tab it was opened from.
	 */
	const session = $derived((page.data as { session: PracticeSession }).session);
	const canChange = $derived(MAY_CHANGE.planTemplates(session));
	const canChangeContractTemplate = $derived(MAY_CHANGE.contractTemplate(session));
	const practiceId = $derived(page.params.practiceId!);

	const outcome = takeOutcome(page.url);

	let planType = $state<PlanType>(planTypeFromQuery(page.url));
	let fields = $state<Field[]>([]);
	let isLoaded = $state(false);
	let error = $state('');
	let tabElements = $state<HTMLButtonElement[]>([]);

	async function load() {
		error = '';
		isLoaded = false;
		try {
			const template = await loadTemplate(apiFetchWithSession, practiceId, planType);
			fields = template.fields;
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load plan template';
		}
	}

	function selectPlanType(value: PlanType) {
		planType = value;
		// The address follows the tab without a navigation, so a reload or
		// a Back from a question lands on the plan type in view.
		const url = new URL(page.url.href);
		url.searchParams.set('plan', value);
		replaceState(url, {});
		void load();
	}

	/*
	 * Arrow keys move focus among the tabs without switching the plan
	 * shown (the WAI-ARIA "manual activation" model, GOV.UK's own Tabs
	 * component's JS layer) -- see #866 for why activation stays on
	 * Enter/Space rather than on focus: selecting a plan type fetches it,
	 * so following focus would fire a request for every tab a keyboard
	 * user passes through on the way to the one she wants.
	 */
	function focusTab(fromIndex: number, direction: 1 | -1) {
		const nextIndex = (fromIndex + direction + PLAN_TYPES.length) % PLAN_TYPES.length;
		tabElements[nextIndex]?.focus();
	}

	function onTabKeydown(event: KeyboardEvent, index: number) {
		if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
			event.preventDefault();
			focusTab(index, 1);
		} else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
			event.preventDefault();
			focusTab(index, -1);
		}
	}

	// One id pair per plan type, so the tab/tabpanel pairing (`id` /
	// `aria-controls` / `aria-labelledby`) is built in one place rather -- spelling:ignore: aria-labelledby is an ARIA attribute name
	// than retyped at each of the three call sites below.
	function tabId(value: PlanType): string {
		return `plan-type-tab-${value}`;
	}
	function panelId(value: PlanType): string {
		return `plan-type-panel-${value}`;
	}

	function changeHref(question: ListedQuestion): string {
		return resolve('/practices/[practiceId]/settings/plan-templates/[planType]/[questionId]', {
			practiceId,
			planType,
			questionId: question.id
		});
	}

	const addHref = $derived(
		resolve('/practices/[practiceId]/settings/plan-templates/[planType]/kind', { practiceId, planType })
	);

	onMount(() => {
		void load();
	});
</script>

<!--
	#865: this screen used to open on a bare field editor. Two sentences
	carry the three things a Practice has to know before touching it, in
	the order she meets them: what the questions are for, that the seeded
	set is hers to change (`staffauth/signup.go` seeds both plan types at
	signup), and the one fact that makes editing safe -- ADR-0001's
	per-instance snapshot, which is why a template edit cannot reach a plan
	already filled in. The word "template" stays out of the prose because
	the heading already says it: #865's own AC is that the copy does not
	restate the page heading.
-->
{#snippet intro()}
	<Text
		text="Every Care Plan and Birth Plan this Practice fills in starts from the questions set here, and the set DoulaCloud seeded is meant to be changed: add, remove and reorder them. A plan already filled in for a Client keeps the questions it was filled in against, so nothing changed here reaches a plan already written."
	/>
{/snippet}

{#snippet planTypeSelector()}
	<!--
		GOV.UK Tabs (docs/design/govuk-alignment.md) -- a mutually exclusive
		choice between two views of the same page, not a route to another
		one. `disabled` used to mark the current type (#866): a disabled
		control drops out of the tab order and is announced as unavailable,
		not as current. `aria-selected` plus a roving `tabindex` keeps both
		buttons reachable and names the current one correctly.
	-->
	<cluster-l space="var(--space-2)" role="tablist" aria-label="Plan type">
		{#each PLAN_TYPES as pt, index (pt.value)}
			<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- Button is a closed atom with no role/aria-selected/tabindex/onkeydown passthrough (ADR-0018); a tab needs all four, and this is the pattern's only consumer -->
			<button
				bind:this={tabElements[index]}
				type="button"
				class="tab"
				role="tab"
				id={tabId(pt.value)}
				aria-selected={planType === pt.value}
				aria-controls={panelId(pt.value)}
				tabindex={planType === pt.value ? 0 : -1}
				onclick={() => selectPlanType(pt.value)}
				onkeydown={(event) => onTabKeydown(event, index)}
			>
				{pt.label}
			</button>
		{/each}
	</cluster-l>
{/snippet}

{#snippet list()}
	<stack-l space="var(--space-5)" role="tabpanel" id={panelId(planType)} aria-labelledby={tabId(planType)} tabindex="0"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
		<!-- #2244: the PUT is OwnerOnly (#1407). Anyone else reads the
			questions, with no Change and no add, and is told why, as the
			Contract Template tells her. -->
		{#if !canChange}
			<Notice variant="info" message="Only a Practice Owner can change these questions." />
		{/if}
		{#if isLoaded}
			<PracticeQuestionList
				questions={fields}
				emptyMessage="No questions yet."
				changeHref={canChange ? changeHref : undefined}
				addHref={canChange ? addHref : undefined}
			/>
		{/if}
	</stack-l>
{/snippet}

{#snippet actions()}
	<!--
		#2193 (MO-G12): a new Practice judges its Plan Templates and then its
		Contract Template, so the way on is here, for someone who may change
		the Contract Template (`MAY_CHANGE`). It is the screen's last thing,
		after the list and its add, so it does not read as one of them.
	-->
	{#if canChangeContractTemplate}
		<Link
			href={resolve('/practices/[practiceId]/settings/contract-template', { practiceId })}
			label="Next, check your Contract Template"
		/>
	{/if}
{/snippet}

<FormPage
	title="Plan Templates"
	{intro}
	{outcome}
	fieldsets={[{ content: planTypeSelector }, { content: list }]}
	{actions}
/>

<style>
	@layer components {
		.tab {
			min-block-size: var(--hit-target-min);
			padding: var(--space-2) var(--space-4);
			border: none;
			border-block-end: var(--border-active) solid transparent;
			background: none;
			color: var(--color-on-surface-variant);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-medium);
			cursor: pointer;
		}

		.tab:hover {
			color: var(--color-primary);
		}

		.tab:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.tab[aria-selected='true'] {
			border-block-end-color: var(--color-primary);
			color: var(--color-primary);
			font-weight: var(--font-weight-semibold);
		}
	}
</style>
