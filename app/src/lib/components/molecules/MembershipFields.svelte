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
		/**
		 * A refusal that belongs to the Roles group rather than to any one
		 * checkbox in it -- "Select at least one role" (#467). It renders
		 * under the legend, which is GOV.UK's position for a group's error
		 * message.
		 */
		rolesError?: string;
		/**
		 * The id given to the *first* role checkbox, so an error summary
		 * entry can link to the group: GOV.UK sends a reader to the first
		 * control of the group that was refused, and the group itself is a
		 * <fieldset>, which is not focusable. Handed in rather than
		 * generated because the route builds the summary and the two have
		 * to agree on one string.
		 */
		rolesFieldId?: string;
	}

	let {
		roles,
		employmentType,
		onRolesChange,
		onEmploymentTypeChange,
		rolesError,
		rolesFieldId
	}: Properties = $props();

	const rolesErrorId = $props.id();

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
	<fieldset aria-describedby={rolesError ? rolesErrorId : undefined} class:refused={Boolean(rolesError)}>
		<legend>Roles</legend>
		{#if rolesError}
			<p id={rolesErrorId} class="error" role="alert">{rolesError}</p>
		{/if}
		<stack-l space="var(--space-5)">
			{#each roleOptions as option, index (option.value)}
				<LabeledField
					id={index === 0 ? rolesFieldId : undefined}
					label={option.label}
					orientation="inline"
				>
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

		/* The group's error message sits between its name and its first
		   option, close to the name it qualifies rather than adrift between
		   the two -- so the legend gives up its own 20px when there is one,
		   and the message keeps it instead. Same treatment `LabeledField`
		   gives a field's own message. */
		fieldset.refused legend {
			margin-block-end: var(--space-1);
		}

		.error {
			margin: 0 0 var(--space-5);
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
