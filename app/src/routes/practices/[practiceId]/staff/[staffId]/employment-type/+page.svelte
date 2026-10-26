<script lang="ts">
	/*
	 * A Staff member's employment type, as one question (#2041). The
	 * Change on the Employment type row of the Staff member's page opens
	 * it. It always opens on the stored answer, so there is nothing to
	 * refuse before the save: a radio group cannot be left empty once one
	 * option is chosen.
	 */
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import {
		EMPLOYMENT_TYPE_DESCRIPTIONS,
		EMPLOYMENT_TYPE_LABELS,
		employmentTypeLabel,
		type EmploymentType
	} from '#lib/roles.js';
	import MembershipQuestion from '../MembershipQuestion.svelte';

	const options = EMPLOYMENT_TYPE_LABELS.map((option) => ({
		...option,
		description: EMPLOYMENT_TYPE_DESCRIPTIONS[option.value]
	}));

	let roles = $state<string[]>([]);
	let employmentType = $state<EmploymentType>('employee');
</script>

<MembershipQuestion
	subject="employment type"
	question={(name) => `Is ${name} an employee or a contractor?`}
	start={(member) => {
		roles = [...member.roles];
		employmentType = member.employmentType;
	}}
	validate={() => []}
	next={() => ({ roles, employmentType })}
	outcome={(member, saved) => `${member.name}'s employment type is now ${employmentTypeLabel(saved.employmentType)}.`}
>
	{#snippet controls()}
		<RadioGroup
			name="membership-employment-type"
			{options}
			value={employmentType}
			onChange={(chosen) => (employmentType = chosen)}
		/>
	{/snippet}
</MembershipQuestion>
