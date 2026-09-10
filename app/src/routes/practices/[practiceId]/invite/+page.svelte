<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import { refusalErrors } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';

	const emailId = 'invite-email';
	const rolesFieldId = 'invite-roles';
	// docs/api-design.md section 7's Details is keyed by the DTO's own
	// JSON field name -- the POST body a few lines below.
	// The employment-type radios share one name, and RadioGroup builds
	// each option's id from it, so the summary entry points at the first
	// one -- GOV.UK's rule for a group, the same shape rolesFieldId takes.
	const employmentTypeName = 'invite-employment-type';
	const employmentTypeFieldId = `${employmentTypeName}-employee`;
	// Keyed by the BFF's own json tags (staffauth.InviteRequest), so a
	// refusal it names lands on the control it is about (#488).
	const inviteFieldIds = {
		email: emailId,
		roles: rolesFieldId,
		employmentType: employmentTypeFieldId
	};

	let email = $state('');
	let roles = $state<string[]>(['doula']);
	let employmentType = $state<'employee' | 'contractor'>('employee');
	const submission = new FormSubmission();
	let invitedAddress = $state('');

	/*
	 * "Select", not "Choose", for a checkbox group -- GOV.UK's own verb for
	 * picking from options on screen, where "Enter" is for typing. The old
	 * wording was "Choose at least one role." with a full stop; an error
	 * message is not a sentence in prose, so it takes neither.
	 */
	function findRefusals(): FormError[] {
		const found: FormError[] = [];
		if (email.trim() === '')
			found.push({ message: "Enter the Staff member's email address", targetId: emailId });
		if (roles.length === 0)
			found.push({ message: 'Select at least one role', targetId: rolesFieldId });
		return found;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		invitedAddress = '';

		await submission.run(async () => {
			const refusals = findRefusals();
			if (refusals.length > 0) {
				return refusals;
			}

			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/invitations`,
				{
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ email, roles, employmentType })
				}
			);
			if (!response.ok) {
				return await refusalErrors(response, inviteFieldIds);
			}

			invitedAddress = email;
			email = '';
			roles = ['doula'];
			employmentType = 'employee';
		}, orServiceProblem);
	}
</script>

<!-- No name field: the Invitation carries an address and a Membership,
     and the person names herself when she accepts. -->
{#snippet who()}
	<LabeledField id={emailId} label="Their email" error={submission.errorFor(emailId)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				type="email"
				value={email}
				onInput={(value) => (email = value)}
				required
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet membership()}
	<MembershipFields
		{roles}
		{employmentType}
		{rolesFieldId}
		{employmentTypeName}
		rolesError={submission.errorFor(rolesFieldId)}
		employmentTypeError={submission.errorFor(employmentTypeFieldId)}
		onRolesChange={(next) => (roles = next)}
		onEmploymentTypeChange={(next) => (employmentType = next)}
	/>
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet actions()}
	<Button type="submit" label="Send invite" loading={submission.isSubmitting} />
	<!-- The accept link is not shown here and never reaches this response:
	     it goes to the invited address only, so accepting is proof she
	     controls that mailbox.

	     The confirmation sits beside the button she just pressed rather
	     than in a banner at the top, the same call `account` made; unlike
	     `account` it stays inside the Template, which owns the page
	     measure and gutters. -->
	{#if invitedAddress}
		<Notice
			variant="status"
			message="Invited. An email with a link to join is on its way to {invitedAddress}."
		/>
	{/if}
{/snippet}

<!--
	Two groups, neither named. GOV.UK's grouping test is whether the fields
	answer one question: "who are you inviting" is the address, and "what
	will she be here" is the Membership. Naming them would print two
	legends the Owner does not need -- the h1 already says what the page
	is -- so they are unnamed groups, which is the case #425 asked this
	Template to handle. Two groups rather than one because the brief puts
	28px between labeled field groups and 20px between consecutive
	fields, and `FormPage`'s outer stack is the 28px one.
-->
<!-- `novalidate`: the page refuses the submit and lists every reason at
     once, rather than the browser stopping at the first empty field (#467). -->
<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `FormPage` stacks each fieldset's content itself, so the `<form>` here owns the submit and arranges nothing; a `StackedForm` would put a second, empty stack around one child. -->
<form onsubmit={handleSubmit} novalidate>
	<FormPage
		title="Invite a Staff member"
		fieldsets={[{ content: who }, { content: membership }]}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		{actions}
	/>
</form>
