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

<!--
	Roles and Employment type are two labelled field groups, so the brief's
	Density section puts 28px between them rather than the 20px a bare
	stack would give. The spacing lives here, not on the two screens that
	compose this, because it is the same decision on both (#425).
-->
<stack-l space="var(--space-7)">
	<fieldset>
		<legend>Roles</legend>
		<stack-l space="var(--space-5)">
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
		</stack-l>
	</fieldset>

	<RadioGroup
		legend="Employment type"
		options={employmentOptions}
		value={employmentType}
		onChange={onEmploymentTypeChange}
	/>
</stack-l>

<style>
	@layer components {
		/* The legend is the group's name; the options below it are
		   consecutive fields, so 20px, per the same Density section.
		   The fieldset stays a block -- a flex fieldset drags its own
		   <legend> in as a flex item. */
		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		legend {
			padding: 0;
			margin-block-end: var(--space-5);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}
	}
</style>
