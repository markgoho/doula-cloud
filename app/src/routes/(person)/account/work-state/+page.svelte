<script lang="ts">
	/*
	 * Your account's work-state question (#2044): the Change on the
	 * summary's "Where you work" row. One question, one select, one
	 * button, and back to the summary with a status Notice (#1710).
	 *
	 * One deliberate act, choose a state, press Save. No confirmation
	 * step, and that is a decision rather than an omission (#437).
	 *
	 * A confirmation dialog buys its friction with a promise that the act
	 * is hard to undo. This one is not: picking the previous state again
	 * puts it back, and both events are recorded either way, so the audit
	 * trail is richer for the round trip rather than damaged by it. And
	 * the failure this screen exists to fix is not a doula who changes
	 * her state carelessly -- it is a doula who moves and never says so,
	 * leaving her Practice's sales tax quietly wrong for years. Friction
	 * here pushes towards that failure, not away from it. The consequence
	 * is stated in the question's one hint instead, before the choice.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { refusalErrors } from '#lib/formErrors.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { WORK_STATE_NAMES, workStateCode, workStateName } from '#lib/workStates.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { loadAccountSession, resetAccountSession } from '../../session.svelte.js';

	const workStateId = 'account-work-state';
	const accountHref = resolve('/(person)/account');

	// The full state name the <select> speaks; workStateCode() converts it
	// back to the USPS code the API stores on the way out.
	let selectedState = $state('');
	const submission = new FormSubmission();

	// The state she reported last, as the answer to change, unless she has
	// already chosen one while the read was in flight.
	onMount(async () => {
		const result = await loadAccountSession();
		if (selectedState !== '' || !result.ok) return;
		selectedState = workStateName(result.session.workState);
	});

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			if (selectedState === '') {
				return [{ message: 'Choose the state you work from', targetId: workStateId }];
			}

			/*
			 * Sent every time, including when the state has not changed.
			 * Saying "yes, still New York, as of today" is a real thing to
			 * say: the reported date is the only staleness signal the design
			 * has, so a re-assertion moves it and is worth having. That is
			 * why the button is never disabled on an unchanged value and the
			 * request is never skipped.
			 *
			 * There is no staffId in the path or the body. The endpoint
			 * only ever writes the caller's own row, which is how self-edit
			 * only is enforced where it can actually be enforced -- an Owner
			 * reads a work state on the roster and cannot write it (#415).
			 */
			const response = await apiFetchWithSession('/api/staff/work-state', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ workState: workStateCode(selectedState) })
			});
			if (!response.ok) {
				return await refusalErrors(response, { workState: workStateId });
			}

			const saved: { workState: string } = await response.json();
			resetAccountSession();
			await gotoWithOutcome(accountHref, `You work from ${workStateName(saved.workState)}.`);
		}, orServiceProblem);
	}
</script>

<!-- Passed only while there is a refusal (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

<!--
	The hint carries the sales-tax line, and nothing else on the page
	explains it (#2044): what the state is for, and the answer to the
	question it invites -- no, a change does not reach backwards, because
	#420 records the tax actually charged on each purchase row.
-->
<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={handleSave} novalidate>
	<QuestionPage
		journey="Changing where you work"
		steps={[]}
		backHref={accountHref}
		question={{ as: 'label', text: 'Which state do you work from?', for: workStateId }}
		hint="We use it for sales tax on your Practice's Credits, from today. Past purchases are not re-priced."
		error={submission.errorFor(workStateId)}
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content({ describedBy })}
			<!-- 20 characters: the longest of the 51 names, "District of Columbia" (#1216). -->
			<Select
				id={workStateId}
				{describedBy}
				invalid={Boolean(submission.errorFor(workStateId))}
				width={20}
				options={[...WORK_STATE_NAMES]}
				placeholder="Choose a state"
				bind:value={selectedState}
				required
			/>
		{/snippet}

		{#snippet actions()}
			<!-- Saving the same state again is a re-assertion, not a no-op;
			     hence no `disabled` on an unchanged value. -->
			<Button type="submit" label="Save where you work" loading={submission.isSubmitting} />
		{/snippet}
	</QuestionPage>
</form>
