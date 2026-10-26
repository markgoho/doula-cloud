<script lang="ts">
	/*
	 * Where she works, on its own page (#2049): the state, for sales tax
	 * on the Practice's Credits (#415), and the Practice's timezone,
	 * opened on the zone the browser reports (#1166).
	 */
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';
	import { TIMEZONE_HINT } from '#lib/timezones.js';
	import SignupQuestion from '../SignupQuestion.svelte';
	import { FIELD_IDS, signupDraft, whereRefusals } from '../signupJourney.svelte.js';
</script>

<SignupQuestion
	stepId="where"
	question={{ as: 'legend', text: 'Where do you work?' }}
	validate={() => whereRefusals(signupDraft.answers)}
>
	{#snippet controls({ errorFor })}
		<stack-l space="var(--space-5)">
			<WorkStateField
				id={FIELD_IDS.workState}
				bind:value={
					() => signupDraft.answers.workStateName, (value) => signupDraft.update({ workStateName: value })
				}
				error={errorFor(FIELD_IDS.workState)}
			/>
			<TimezoneField
				id={FIELD_IDS.timezone}
				bind:value={() => signupDraft.answers.timezone, (value) => signupDraft.update({ timezone: value })}
				error={errorFor(FIELD_IDS.timezone)}
				hint={TIMEZONE_HINT}
			/>
		</stack-l>
	{/snippet}
</SignupQuestion>
