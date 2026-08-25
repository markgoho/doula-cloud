<script lang="ts">
	import LabeledField from './LabeledField.svelte';
	import RadioGroup from './RadioGroup.svelte';
	import Checkbox from '../atoms/Checkbox.svelte';

	// The two halves of a Membership, edited together: ADR-0008 makes
	// roles what a person does at a Practice and employment type what she
	// is to the business. They are one component because they are one
	// decision -- an Owner picks them together when she invites somebody
	// (#266) and again when she edits an existing Membership (#261), and
	// the two screens must not drift apart.
	interface Properties {
		roles: string[];
		employmentType: 'employee' | 'contractor';
		onRolesChange: (roles: string[]) => void;
		onEmploymentTypeChange: (employmentType: 'employee' | 'contractor') => void;
	}

	let { roles, employmentType, onRolesChange, onEmploymentTypeChange }: Properties = $props();

	const roleOptions: { value: string; label: string }[] = [
		{ value: 'owner', label: 'Owner' },
		{ value: 'admin', label: 'Admin' },
		{ value: 'doula', label: 'Doula' }
	];

	const employmentOptions: { value: 'employee' | 'contractor'; label: string }[] = [
		{ value: 'employee', label: 'Employee' },
		{ value: 'contractor', label: 'Contractor' }
	];

	function toggleRole(role: string, isChecked: boolean) {
		// Order follows roleOptions rather than the click order, so the
		// same set of roles always reads the same way.
		onRolesChange(
			isChecked
				? roleOptions.map((option) => option.value).filter((value) => value === role || roles.includes(value))
				: roles.filter((value) => value !== role)
		);
	}
</script>

<fieldset>
	<legend>Roles</legend>
	{#each roleOptions as option (option.value)}
		<LabeledField label={option.label} orientation="inline">
			{#snippet children(control)}
				<Checkbox
					checked={roles.includes(option.value)}
					onChange={(isChecked) => toggleRole(option.value, isChecked)}
					{...control}
				/>
			{/snippet}
		</LabeledField>
	{/each}
</fieldset>

<RadioGroup
	legend="Employment type"
	options={employmentOptions}
	value={employmentType}
	onChange={onEmploymentTypeChange}
/>
