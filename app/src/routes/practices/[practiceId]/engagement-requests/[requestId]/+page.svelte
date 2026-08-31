<script lang="ts">
	/*
	 * The approval screen (#502, ADR-0017): one pending Engagement Request,
	 * everything the approver needs to decide it, and the two decisions
	 * themselves. Addressed by the Request's own id and nothing else, so it
	 * is reachable before an inbox exists to list them (#503).
	 *
	 * There is no control that amends the kind or the due date. ADR-0017:
	 * "the requester describes the work; the approver does not amend it" --
	 * so both are rendered as facts in a DescriptionList, never as inputs.
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
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		approveRequest,
		formatCalendarDay,
		formatInstant,
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
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';

	const reasonId = 'engagement-request-refusal-reason';

	let detail = $state<ApprovalDetail | undefined>();
	let loadError = $state('');
	let reason = $state('');
	let errors = $state<FormError[]>([]);
	let isApproving = $state(false);
	let isRefusing = $state(false);
	let hasNoCredits = $state(false);

	const isDeciding = $derived(isApproving || isRefusing);
	// The empty-balance path is offered before the attempt as well as after
	// it: the read already knows the balance, so an approver is told she
	// must buy Credits rather than discovering it by pressing Approve.
	const isBalanceEmpty = $derived(detail !== undefined && detail.balanceAfter < 0);

	function clientName(record: ApprovalDetail['client']): string {
		if (record.preferredName) return record.preferredName;
		return [record.givenName, record.familyName].filter(Boolean).join(' ');
	}

	function facts(request: ApprovalDetail): { label: string; value: string }[] {
		return [
			{
				label: 'Client',
				value: `${clientName(request.client)} -- ${request.client.isNewToPractice ? 'new to this practice' : 'already known here'}`
			},
			{ label: 'Asked by', value: `${request.requestedByName} on ${formatInstant(request.requestedAt)}` },
			{ label: 'Kind of work', value: kindLabel(request.kind) },
			{ label: 'Due date', value: request.dueDate ? formatCalendarDay(request.dueDate) : 'Not given' },
			{ label: 'Note', value: request.note ?? 'None' },
			{ label: 'Credit cost', value: `${request.creditCost} credit` },
			{ label: 'Balance after', value: String(request.balanceAfter) }
		];
	}

	function engagementLabel(engagement: ApprovalDetail['engagements'][number]): string {
		const kind = kindLabel(engagement.kind);
		return `${kind} work, started ${formatInstant(engagement.createdAt)} -- ${engagement.status}`;
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
		if (hasNoCredits || isBalanceEmpty) rememberApprovalReturn(approvalHref());
	});

	onMount(async () => {
		forgetApprovalReturn();
		try {
			detail = await loadApprovalDetail(apiFetchWithSession, page.params.practiceId!, page.params.requestId!);
		} catch (error_) {
			loadError = error_ instanceof Error && error_.message ? error_.message : 'Failed to load the request';
		}
	});

	async function handleApprove() {
		errors = [];
		hasNoCredits = false;
		isApproving = true;
		try {
			const result = await approveRequest(apiFetchWithSession, page.params.practiceId!, page.params.requestId!);
			if (result.noCredits) {
				hasNoCredits = true;
				return;
			}
			await goto(engagementHref(result.outcome.engagementId));
		} catch (error_) {
			errors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isApproving = false;
		}
	}

	async function handleRefuse(event: SubmitEvent) {
		event.preventDefault();
		errors = [];
		hasNoCredits = false;
		if (reason.trim() === '') {
			errors = [{ message: 'Enter why this request is being refused', targetId: reasonId }];
			return;
		}
		isRefusing = true;
		try {
			await refuseRequest(apiFetchWithSession, page.params.practiceId!, page.params.requestId!, reason.trim());
			await goto(clientHref(detail!.client.clientId));
		} catch (error_) {
			errors = [{ message: error_ instanceof Error && error_.message ? error_.message : SERVICE_PROBLEM }];
		} finally {
			isRefusing = false;
		}
	}

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}
</script>

{#snippet summary()}
	{#if detail}
		<stack-l space="var(--space-4)">
			{#if detail.warning}
				<Notice
					variant="info"
					message="{clientName(detail.client)} already has a live engagement. That does not stop this approval."
				/>
			{/if}
			{#if hasNoCredits || isBalanceEmpty}
				<Notice variant="error" message="There are no credits left on this practice's balance." />
				<Link href={billingHref()} label="Buy credits" />
			{/if}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
			<DescriptionList items={facts(detail)} />
			<Link href={clientHref(detail.client.clientId)} label="View {clientName(detail.client)}'s record" />
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
		<Text text="This would be her first engagement with this practice." />
	{/if}
{/snippet}

{#snippet decideSection()}
	<stack-l space="var(--space-5)">
		<Button label="Approve and start the work" loading={isApproving} disabled={isDeciding} onClick={handleApprove} />
		<form onsubmit={handleRefuse} novalidate>
			<stack-l space="var(--space-4)">
				<LabeledField
					id={reasonId}
					label="Why are you refusing this?"
					hint="The doula who asked will see this."
					error={errorFor(reasonId)}
				>
					{#snippet children({ id, describedBy, invalid })}
						<Textarea {id} {describedBy} {invalid} value={reason} onInput={(v) => (reason = v)} />
					{/snippet}
				</LabeledField>
				<Button
					type="submit"
					variant="destructive"
					label="Refuse this request"
					loading={isRefusing}
					disabled={isDeciding}
				/>
			</stack-l>
		</form>
	</stack-l>
{/snippet}

<RecordDetail
	title={detail ? `Approve work with ${clientName(detail.client)}` : 'Approve an engagement request'}
	summary={detail ? summary : undefined}
	sections={detail
		? [
				{ heading: 'Her engagements', content: engagementsSection },
				{ heading: 'Decide', content: decideSection }
			]
		: []}
	loading={detail || loadError ? undefined : 'Loading the request'}
	loadError={loadError || undefined}
/>
