<script lang="ts">
	import CheckboxGroup from './CheckboxGroup.svelte';
	import RadioGroup from './RadioGroup.svelte';
	import {
		ROLE_LABELS,
		ROLE_DESCRIPTIONS,
		EMPLOYMENT_TYPE_LABELS,
		EMPLOYMENT_TYPE_DESCRIPTIONS,
		type EmploymentType
	} from '#lib/roles.js';

	// The two halves of a Membership, edited together: ADR-0008 makes
	// roles what a person does at a Practice and employment type what she
	// is to the business. They are one component because they are one
	// decision -- an Owner picks them together when she invites somebody
	// (#266) and again when she edits an existing Membership (#261), and
	// the two screens must not drift apart.
	interface Properties {
		roles: string[];
		/**
		 * `undefined` chooses nothing. The invite form asks it as a
		 * question about somebody else, so it opens empty (#1912,
		 * ADR-0055); the edit form passes the stored answer.
		 */
		employmentType: EmploymentType | undefined;
		onRolesChange: (roles: string[]) => void;
		onEmploymentTypeChange: (employmentType: EmploymentType) => void;
		/**
		 * A refusal that belongs to the Roles group rather than to any one
		 * checkbox in it -- "Select at least one role" (#467). It renders
		 * under the legend, which is GOV.UK's position for a group's error
		 * message.
		 */
		rolesError?: string;
		/**
		 * The `name` the role checkboxes share, which is also what their
		 * ids are built from (`checkboxFieldId(rolesName, 'owner')`). Handed
		 * in because the route builds the error summary, which sends a
		 * reader to the group's first box -- GOV.UK's rule, since the
		 * <fieldset> itself is not focusable -- and the two have to agree
		 * on one string.
		 */
		rolesName?: string;
		/**
		 * The `name` the employment-type radios share, which is also what
		 * their ids are built from (`<name>-employee`). Handed in for the
		 * same reason `rolesName` is (#488).
		 */
		employmentTypeName?: string;
		/**
		 * A refusal that belongs to the employment-type group, rendered
		 * under its legend the way `rolesError` is.
		 */
		employmentTypeError?: string;
	}

	let {
		roles,
		employmentType,
		onRolesChange,
		onEmploymentTypeChange,
		rolesError,
		rolesName,
		employmentTypeName,
		employmentTypeError
	}: Properties = $props();

	/* One line under each option (#2014): Owner, Admin and
	   Contractor are the answers a person needs explained. */
	const roleOptions = ROLE_LABELS.map((option) => ({ ...option, description: ROLE_DESCRIPTIONS[option.value] }));

	const employmentOptions = EMPLOYMENT_TYPE_LABELS.map((option) => ({
		...option,
		description: EMPLOYMENT_TYPE_DESCRIPTIONS[option.value]
	}));
</script>

<!--
	Roles and Employment type are two labeled field groups, so the brief's
	Density section puts 28px between them rather than the 20px a bare
	stack would give. The spacing lives here, not on the two screens that
	compose this, because it is the same decision on both (#425).
-->
<stack-l space="var(--space-7)">
	<!-- CheckboxGroup orders the chosen roles as the options are listed,
	     not in the click order, so the same set always reads the same way. -->
	<CheckboxGroup
		legend="Roles"
		name={rolesName}
		options={roleOptions}
		value={roles}
		onChange={onRolesChange}
		error={rolesError}
	/>

	<RadioGroup
		legend="Employment type"
		name={employmentTypeName}
		options={employmentOptions}
		value={employmentType}
		onChange={onEmploymentTypeChange}
		error={employmentTypeError}
	/>
</stack-l>
