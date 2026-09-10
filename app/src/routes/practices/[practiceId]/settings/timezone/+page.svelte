<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadPracticeTimezone, savePracticeTimezone } from '#lib/practiceTimezone.js';
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	const timezoneId = 'practice-timezone';

	let timezone = $state('');
	let fieldError = $state('');
	let loadError = $state('');
	let isSaved = $state(false);

	async function load() {
		loadError = '';
		isSaved = false;
		try {
			const current = await loadPracticeTimezone(apiFetchWithSession, page.params.practiceId!);
			timezone = current.timezone;
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load the timezone';
		}
	}

	async function save() {
		isSaved = false;
		fieldError = '';
		if (timezone === '') {
			fieldError = 'Choose the timezone this Practice works in';
			return;
		}
		try {
			const saved = await savePracticeTimezone(
				apiFetchWithSession,
				page.params.practiceId!,
				timezone
			);
			timezone = saved.timezone;
			isSaved = true;
		} catch (error) {
			fieldError = error instanceof Error ? error.message : 'Failed to save';
		}
	}

	onMount(() => {
		void load();
	});
</script>

{#snippet fields()}
	{#if loadError}
		<Notice variant="error" message={loadError} />
	{/if}
	{#if isSaved}
		<Text text="Saved." />
	{/if}
	<TimezoneField
		id={timezoneId}
		bind:value={timezone}
		error={fieldError}
		hint="Doula Cloud works out which day a Visit falls on in this timezone — which is what decides whether a Visit counts as birth or postpartum work."
	/>
	<!--
		GOV.UK's rule for a consequence a person can still act on: it is
		stated before the press, not reported after it. Changing the zone
		is retroactive by construction -- CONTEXT.md's Visit entry derives
		a Visit's type on every read, so nothing is stored to migrate and
		every Visit already recorded is re-judged against the new day the
		moment this is saved. No instant moves; what moves is which
		calendar day an evening Visit belongs to.
	-->
	<WarningText
		message="Changing this changes how Visits already recorded are typed. A Visit worked late in the evening can move between birth and postpartum work, because the day it falls on is worked out in this timezone."
	/>
{/snippet}

{#snippet actions()}
	<Button label="Save" onClick={save} />
{/snippet}

<FormPage title="Timezone" fieldsets={[{ content: fields }]} {actions} />
