<script lang="ts">
	/**
	 * An Engagement's Doula fees (#2271): each contractor's accepted fee,
	 * whether the Practice has recorded paying it, and the two acts an
	 * Owner or Admin takes on one: Mark fee paid, and Remove payment for a
	 * record made by mistake. A record only. The money moves outside
	 * DoulaCloud (#391, ADR-0007).
	 *
	 * An ended Attachment is listed too: a Practice usually pays after the
	 * birth, by when the Doula is no longer on the Engagement.
	 *
	 * The two forms are the ones a Client's Payment uses (RecordPaymentForm,
	 * ReversePaymentForm). onRecord and onRemove own the API calls.
	 */
	import { formatAmount, type RecordPaymentInput } from '#lib/invoice.js';
	import { feePaidLine, type DoulaFee } from '#lib/doulaFee.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import RecordPaymentForm from '#lib/components/organisms/RecordPaymentForm.svelte';
	import ReversePaymentForm from '#lib/components/organisms/ReversePaymentForm.svelte';

	let {
		fees,
		practiceTimezone,
		onRecord,
		onRemove
	}: {
		fees: DoulaFee[];
		/**
		The Practice's own IANA zone (ADR-0036), for the date paid.
		*/
		practiceTimezone: string;
		onRecord: (attachmentId: string, input: RecordPaymentInput) => Promise<void>;
		onRemove: (attachmentId: string, paymentId: string, reason: string) => Promise<void>;
	} = $props();

	type OpenForm = { kind: 'record'; attachmentId: string } | { kind: 'remove'; attachmentId: string; paymentId: string };
	let openForm = $state<OpenForm | undefined>();
	// What the last act wrote; it takes focus, since the form is gone.
	let status = $state('');

	const openFee = $derived(fees.find((fee) => fee.attachmentId === openForm?.attachmentId));

	function open(form: OpenForm) {
		status = '';
		openForm = form;
	}

	function close() {
		openForm = undefined;
	}

	async function record(fee: DoulaFee, input: RecordPaymentInput) {
		await onRecord(fee.attachmentId, input);
		close();
		status = `Recorded: ${fee.doulaName} paid ${feePaidLine({ amountCents: fee.amountCents, ...input })}.`;
	}

	async function remove(fee: DoulaFee, paymentId: string, reason: string) {
		await onRemove(fee.attachmentId, paymentId, reason);
		close();
		status = `Payment to ${fee.doulaName} removed.`;
	}

	function rowId(fee: DoulaFee, view: DataTableView): string {
		return `doula-fee-${view}-${fee.attachmentId}`;
	}

	const allColumns = [
		{ label: 'Doula', accessor: (fee: DoulaFee) => fee.doulaName },
		{ label: 'Fee', accessor: (fee: DoulaFee) => formatAmount(fee.amountCents), numeric: true },
		{
			label: 'On this birth',
			accessor: (fee: DoulaFee) =>
				fee.endedOn
					? `${formatCalendarDay(fee.acceptedOn)} to ${formatCalendarDay(fee.endedOn)}`
					: `Since ${formatCalendarDay(fee.acceptedOn)}`
		},
		{ label: 'Paid', accessor: (fee: DoulaFee) => (fee.paid ? feePaidLine(fee.paid) : 'Not recorded yet'), wraps: true },
		{ label: 'Recorded by', accessor: (fee: DoulaFee) => fee.paid?.recordedBy ?? '' },
		{ label: 'Note', accessor: (fee: DoulaFee) => fee.paid?.note ?? '', variant: 'muted' as const, wraps: true }
	];
	// A Note column with nothing in it squeezes its own header to one
	// letter a line, so it is drawn only when some record carries a note.
	const columns = $derived(fees.some((fee) => fee.paid?.note) ? allColumns : allColumns.slice(0, -1));
</script>

{#snippet feeActions(fee: DoulaFee, view: DataTableView)}
	{@const nameId = `doula-fee-name-${view}-${fee.attachmentId}`}
	<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- both values are required strings -->
	<span class="visually-hidden" id={nameId}>{fee.doulaName}, {formatAmount(fee.amountCents)}</span>
	<!-- v8 ignore stop -->
	{#if fee.paid}
		{@const paymentId = fee.paid.id}
		<Button
			label="Remove payment"
			variant="secondary"
			size="sm"
			describedBy={nameId}
			onClick={() => open({ kind: 'remove', attachmentId: fee.attachmentId, paymentId })}
		/>
	{:else}
		<Button
			label="Mark fee paid"
			variant="secondary"
			size="sm"
			describedBy={nameId}
			onClick={() => open({ kind: 'record', attachmentId: fee.attachmentId })}
		/>
	{/if}
{/snippet}

{#if status}
	<Notice variant="status" message={status} isFocusedOnAppear />
{/if}

<DataTable
	{columns}
	rows={fees}
	{rowId}
	rowActions={{ label: 'Actions', content: feeActions }}
	emptyMessage="No Doula on this Engagement has an agreed fee."
/>

{#if openForm && openFee}
	{@const fee = openFee}
	{@const form = openForm}
	{#key form}
		{#if form.kind === 'record'}
			<RecordPaymentForm
				label="Record a fee paid"
				heading="Record paying {fee.doulaName} {formatAmount(fee.amountCents)}"
				amountCents={fee.amountCents}
				dateLabel="Date paid"
				dateMissingMessage="Enter the date the fee was paid"
				{practiceTimezone}
				onConfirm={(input) => record(fee, input)}
				onCancel={close}
			/>
		{:else}
			<ReversePaymentForm
				label="Remove this fee payment"
				heading="Remove the {formatAmount(fee.amountCents)} paid to {fee.doulaName}"
				amountCents={fee.amountCents}
				reasonMissingMessage="Enter a reason for removing this payment"
				confirmLabel="Confirm and remove"
				onConfirm={(reason) => remove(fee, form.paymentId, reason)}
				onCancel={close}
			/>
		{/if}
	{/key}
{/if}
