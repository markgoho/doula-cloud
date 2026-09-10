<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import DynamicFieldEditor from '#lib/components/organisms/DynamicFieldEditor.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import {
		loadTemplate,
		saveTemplate,
		addField,
		removeField,
		moveField,
		validateFields,
		type Field,
		type FieldType
	} from '#lib/planTemplate.js';

	const planTypes = [
		{ value: 'care_plan', label: 'Care Plan' },
		{ value: 'birth_plan', label: 'Birth Plan' }
	] as const;

	let planType = $state<'care_plan' | 'birth_plan'>('care_plan');
	let fields = $state<Field[]>([]);
	let error = $state('');
	let isSaved = $state(false);
	let tabElements = $state<HTMLButtonElement[]>([]);

	async function load() {
		error = '';
		isSaved = false;
		try {
			const template = await loadTemplate(apiFetchWithSession, page.params.practiceId!, planType);
			fields = template.fields;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load plan template';
		}
	}

	async function save() {
		error = '';
		isSaved = false;
		const validationError = validateFields(fields);
		if (validationError) {
			error = validationError;
			return;
		}
		try {
			const template = await saveTemplate(apiFetchWithSession, page.params.practiceId!, planType, fields);
			fields = template.fields;
			isSaved = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to save plan template';
		}
	}

	function selectPlanType(value: 'care_plan' | 'birth_plan') {
		planType = value;
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
		const nextIndex = (fromIndex + direction + planTypes.length) % planTypes.length;
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
	function tabId(value: 'care_plan' | 'birth_plan'): string {
		return `plan-type-tab-${value}`;
	}
	function panelId(value: 'care_plan' | 'birth_plan'): string {
		return `plan-type-panel-${value}`;
	}

	function updateField(id: string, patch: Partial<Field>) {
		fields = fields.map((f) => (f.id === id ? { ...f, ...patch } : f));
	}

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
		text="Every Care Plan and Birth Plan this Practice fills in starts from the questions set here, and the set Doula Cloud seeded is meant to be changed: add, remove and reorder them. A plan already filled in for a Client keeps the questions it was filled in against, so nothing changed here reaches a plan already written."
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
		{#each planTypes as pt, index (pt.value)}
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

{#snippet editor()}
	<div role="tabpanel" id={panelId(planType)} aria-labelledby={tabId(planType)} tabindex="0"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
		{#if error}
			<Notice variant="error" message={error} />
		{/if}
		{#if isSaved}
			<Text text="Saved." />
		{/if}

		<DynamicFieldEditor
			{fields}
			onAdd={(type: FieldType) => (fields = addField(fields, crypto.randomUUID(), type))}
			onRemove={(id: string) => (fields = removeField(fields, id))}
			onMoveUp={(id: string) => (fields = moveField(fields, id, 'up'))}
			onMoveDown={(id: string) => (fields = moveField(fields, id, 'down'))}
			onLabelChange={(id: string, label: string) => updateField(id, { label })}
			onTypeChange={(id: string, type: FieldType) => updateField(id, { type })}
			onOptionsChange={(id: string, options: string[]) => updateField(id, { options })}
		/>
	</div>
{/snippet}

{#snippet actions()}
	<Button label="Save" onClick={save} />
{/snippet}

<FormPage
	title="Plan Templates"
	{intro}
	fieldsets={[{ content: planTypeSelector }, { content: editor }]}
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
