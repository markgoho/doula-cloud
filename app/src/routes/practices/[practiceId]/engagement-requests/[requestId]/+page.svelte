<script lang="ts">
	/*
	 * The approval screen (#502, ADR-0017): one pending Engagement Request,
	 * everything the approver needs to decide it, and the two decisions
	 * themselves. Addressed by the Request's own id and nothing else, so it
	 * is reachable before an inbox exists to list them (#503).
	 *
	 * There is no control that amends the kind, the due date or the named
	 * Doula (#1596). ADR-0017: "the requester describes the work; the
	 * approver does not amend it" -- so all three are rendered as facts in
	 * a SummaryList, never as inputs.
	 * An approver who disagrees with the ask refuses it and says why.
	 *
	 * Refuse demands a reason, and this screen refuses an empty one before
	 * it ever calls the endpoint. The endpoint and
	 * engagement_requests_refusal_reason both enforce it, but a round trip
	 * that comes back "reason is required" as a bare 400 is a worse way to
	 * learn it than an error summary that moves focus to the field.
	 *
	 * No confirm dialog stands in front of Approve, even though spending a
	 * Credit is irreversible. This whole screen is the confirmation step --
	 * a second "are you sure" over a page whose only purpose is to decide
	 * is the double-confirm GOV.UK's own guidance warns against.
	 *
	 * "Nothing lost" after a Buy Credits round trip (AC6) needs no draft
	 * storage here, unlike the request form: every fact on this screen
	 * derives from the Request id in the URL, so coming back to the same
	 * address rebuilds it. The one piece of typed state is the refusal
	 * reason, and refusing never runs out of Credits. What the round trip
	 * does need is the way back -- Stripe returns to the Billing page and
	 * nowhere else -- so this screen remembers its own address on the way
	 * out and forgets it on the way in (see engagementRequest.ts).
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay, formatInstant } from '#lib/dates.js';
	import { engagementStatusText } from '#lib/engagementDetail.js';
	import { firstName, fullName } from '#lib/personName.js';
	import {
		NO_DOULA_YET_LABEL,
		approveRequest,
		kindLabel,
		loadApprovalDetail,
		forgetApprovalReturn,
		refuseRequest,
		rememberApprovalReturn,
		type ApprovalDetail
	} from '#lib/engagementRequest.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import { FormSubmission, orThrownMessage, type FormError } from '#lib/formSubmission.svelte.js';

	const reasonId = 'engagement-request-refusal-reason';
	const decisionName = 'engagement-request-decision';
	// The summary entry for the question points at its first option,
	// GOV.UK's rule for a group.
	const decisionFieldId = radioFieldId(decisionName, 'approve');

	let detail = $state<ApprovalDetail | undefined>();
	let loadError = $state('');
	/*
	 * #2043: the two decisions are two answers to one question, "Do you
	 * approve this work?", with one Continue -- the Birth outcome shape.
	 * It opens with neither chosen (ADR-0055): Approve spends a Credit,
	 * and a Continue pressed without reading must not spend it.
	 */
	let decision = $state<'approve' | 'refuse' | undefined>();
	let reason = $state('');
	const submission = new FormSubmission();
	// The empty-balance path is offered before the attempt as well as after
	// it: the read already knows the balance, so an approver is told she
	// must buy Credits rather than discovering it by pressing Approve.
	// `balanceAfter` already has the one-Credit cost subtracted server-side
	// (engagementrequest.DetailResponse), so "empty" is `< 0` here rather
	// than the request form's own `balance <= 0` -- the same one-Credit
	// assumption, read off a value that already has it applied.
	const isBalanceEmpty = $derived(detail !== undefined && detail.balanceAfter < 0);
	// #2284, as #2186 did on the Start work form: a press the balance
	// refuses is an entry in the error summary, which takes focus and
	// announces it, and the entry links to the Credits screen, where the
	// fix is. While the summary says it, the notice the screen opened with
	// does not say it a second time beside it.
	const isCreditRefusalShown = $derived(submission.errors.some((entry) => entry.href === billingHref()));
	const isBalanceNoticeShown = $derived(isBalanceEmpty && !isCreditRefusalShown);

	function creditRefusal(): FormError {
		return {
			message: `Buy credits to approve work with ${firstName(detail!.client)}. This Practice has none left.`,
			href: billingHref()
		};
	}

	function facts(request: ApprovalDetail): { label: string; value: string }[] {
		const items = [
			{
				label: 'Client',
				value: `${fullName(request.client)} — ${request.client.isNewToPractice ? 'new to this Practice' : 'already known here'}`
			},
			{ label: 'Asked by', value: `${request.requestedByName} on ${formatInstant(request.requestedAt)}` },
			{ label: 'Kind of work', value: kindLabel(request.kind) },
			{ label: 'Due date', value: request.dueDate ? formatCalendarDay(request.dueDate) : 'Not given' },
			// #1596: the third fact the asker states. A fact and never a
			// control, like the kind and the due date beside it.
			{ label: 'Doula', value: request.doulaName ?? NO_DOULA_YET_LABEL },
			{ label: 'Note', value: request.note ?? 'None' },
			{ label: 'Credit cost', value: `${request.creditCost} credit` }
		];
		// `balanceAfter` is honestly negative on an empty balance
		// (engagementRequest.ts) so `isBalanceEmpty`'s own Notice can be
		// driven off it -- but that makes it unfit to print here too. The
		// line is omitted rather than floored (#1235): the Notice right
		// beside it already says there is nothing to spend.
		if (request.balanceAfter >= 0) {
			items.push({ label: 'Balance after', value: String(request.balanceAfter) });
		}
		return items;
	}

	function engagementLabel(engagement: ApprovalDetail['engagements'][number]): string {
		const kind = kindLabel(engagement.kind);
		return `${kind} work, started ${formatInstant(engagement.createdAt)} — ${engagementStatusText(engagement.status)}`;
	}

	function clientHref(clientId: string): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId
		});
	}

	function engagementHref(engagementId: string): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId
		});
	}

	function billingHref(): string {
		return resolve('/practices/[practiceId]/billing', { practiceId: page.params.practiceId! });
	}

	function approvalHref(): string {
		return resolve('/practices/[practiceId]/engagement-requests/[requestId]', {
			practiceId: page.params.practiceId!,
			requestId: page.params.requestId!
		});
	}

	// Remembered only while the way back is actually being offered, so a
	// reader who never runs out of Credits never touches storage at all.
	$effect(() => {
		if (isBalanceEmpty) rememberApprovalReturn(approvalHref());
	});

	onMount(async () => {
		forgetApprovalReturn();
		try {
			detail = await loadApprovalDetail(apiFetchWithSession, page.params.practiceId!, page.params.requestId!);
		} catch (error_) {
			loadError = error_ instanceof Error && error_.message ? error_.message : 'Failed to load the request';
		}
	});

	/*
	 * What Approve costs, as the hint under it: the Credit it spends and
	 * what is left. On an empty balance the negative arithmetic stays off
	 * the screen (#1235); the Notice above already says there is none.
	 */
	function approveHint(request: ApprovalDetail): string {
		const cost = `Uses ${request.creditCost} credit.`;
		return request.balanceAfter < 0
			? `${cost} There are none left.`
			: `${cost} ${request.balanceAfter} left after this.`;
	}

	async function handleDecide(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			if (decision === undefined) {
				return [{ message: 'Select whether you approve this work', targetId: decisionFieldId }];
			}
			if (decision === 'refuse') {
				if (reason.trim() === '') {
					return [{ message: 'Enter why this request is being refused', targetId: reasonId }];
				}
				await refuseRequest(apiFetchWithSession, page.params.practiceId!, page.params.requestId!, reason.trim());
				await goto(clientHref(detail!.client.clientId));
				return;
			}
			const result = await approveRequest(apiFetchWithSession, page.params.practiceId!, page.params.requestId!);
			if (result.noCredits) {
				// The BFF is what refuses, even on a balance the screen
				// already read as empty: Credits bought in another tab since
				// it opened would make that read stale. In the race (a
				// Credit at load, none at the press) the balance is read as
				// empty now, which takes the stale "Balance after" off the
				// screen and remembers the way back from buying Credits.
				detail!.balanceAfter = -detail!.creditCost;
				return [creditRefusal()];
			}
			await goto(engagementHref(result.outcome.engagementId));
		}, orThrownMessage);
	}
</script>

{#snippet summary()}
	{#if detail}
		<stack-l space="var(--space-4)">
			{#if detail.warning}
				<Notice
					variant="info"
					message="{firstName(detail.client)} already has a live engagement. That does not stop this approval."
				/>
			{/if}
			{#if isBalanceNoticeShown}
				<Notice variant="error" message="There are no Credits left on this Practice's balance." />
				<Link href={billingHref()} label="Buy credits" />
			{/if}
			{#if submission.errors.length > 0}
				<ErrorSummary errors={submission.errors} />
			{/if}
			<SummaryList rows={facts(detail)} />
			<Link href={clientHref(detail.client.clientId)} label="View {firstName(detail.client)}'s record" />
		</stack-l>
	{/if}
{/snippet}

{#snippet engagementsSection()}
	{#if detail && detail.engagements.length > 0}
		<ul>
			{#each detail.engagements as engagement (engagement.engagementId)}
				<li>
					<Link href={engagementHref(engagement.engagementId)} label={engagementLabel(engagement)} />
				</li>
			{/each}
		</ul>
	{:else}
		<Text text={`This would be ${firstName(detail!.client)}'s first engagement with this Practice.`} />
	{/if}
{/snippet}

{#snippet refusalReason()}
	<LabeledField
		id={reasonId}
		label="Why are you refusing this?"
		hint="The doula who asked will see this."
		error={submission.errorFor(reasonId)}
	>
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={reason} onInput={(v) => (reason = v)} />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet decideSection()}
	<!--
		#2043: one question, two answers, one primary -- the Birth outcome
		shape. The reason is asked only under Refuse, so the destructive
		answer is no longer a whole second form a hand's width from Approve.
	-->
	<!-- A form column, as a FormPage would give it: RecordDetail's section
	     is the page's whole width, and the reason box ran 1,300px wide at
	     1440 (F-B-11). -->
	<div class="decide">
		<StackedForm onSubmit={handleDecide}>
			<RadioGroup
				legend="Do you approve this work?"
				name={decisionName}
				options={[
					{ value: 'approve', label: 'Approve and start the work', description: approveHint(detail!) },
					{ value: 'refuse', label: 'Refuse', conditional: refusalReason }
				]}
				value={decision}
				onChange={(value) => (decision = value)}
				error={submission.errorFor(decisionFieldId)}
			/>
			<Button type="submit" label="Continue" loading={submission.isSubmitting} />
		</StackedForm>
	</div>
{/snippet}

<RecordDetail
	title={detail ? `Approve work with ${fullName(detail.client)}` : 'Approve an Engagement Request'}
	summary={detail ? summary : undefined}
	sections={detail
		? [
				{ heading: `${firstName(detail.client)}'s engagements`, content: engagementsSection },
				{ heading: 'Decide', content: decideSection }
			]
		: []}
	loading={detail || loadError ? undefined : 'Loading the request'}
	loadError={loadError || undefined}
/>

<style>
	@layer components {
		.decide {
			max-inline-size: var(--form-max);
		}
	}
</style>
