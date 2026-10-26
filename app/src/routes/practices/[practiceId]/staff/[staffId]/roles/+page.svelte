<script lang="ts">
	/*
	 * A Staff member's roles, as one question (#2041). The Change on the
	 * Roles row of the Staff member's page opens it.
	 */
	import CheckboxGroup, { checkboxFieldId } from '#lib/components/molecules/CheckboxGroup.svelte';
	import { ROLE_DESCRIPTIONS, ROLE_LABELS, rolesLabel, type EmploymentType } from '#lib/roles.js';
	import type { FormError } from '#lib/formSubmission.svelte.js';
	import MembershipQuestion from '../MembershipQuestion.svelte';

	const rolesName = 'membership-roles';
	// GOV.UK sends a group's error to its first box: a <fieldset> takes no
	// focus.
	const rolesFieldId = checkboxFieldId(rolesName, 'owner');

	const options = ROLE_LABELS.map((option) => ({ ...option, description: ROLE_DESCRIPTIONS[option.value] }));

	let roles = $state<string[]>([]);
	let employmentType = $state<EmploymentType>('employee');

	// A Membership always holds at least one role (membership.go refuses
	// an empty set), so the page says so before it asks the server.
	function validate(): FormError[] {
		return roles.length === 0 ? [{ message: 'Select at least one role', targetId: rolesFieldId }] : [];
	}
</script>

<MembershipQuestion
	subject="roles"
	question={(name) => `Which roles does ${name} have?`}
	start={(member) => {
		roles = [...member.roles];
		employmentType = member.employmentType;
	}}
	{validate}
	next={() => ({ roles, employmentType })}
	outcome={(member, saved) => `${member.name}'s roles are now ${rolesLabel(saved.roles)}.`}
>
	{#snippet controls(submission)}
		<CheckboxGroup
			name={rolesName}
			{options}
			value={roles}
			onChange={(chosen) => (roles = chosen)}
			error={submission.errorFor(rolesFieldId)}
		/>
	{/snippet}
</MembershipQuestion>
