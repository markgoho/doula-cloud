<script lang="ts">
	/*
	 * Where Staff see the addresses Doula Cloud has stopped writing to,
	 * and lift the ones that can be lifted (ADR-0029, #744).
	 *
	 * An address-keyed list, not a per-record affordance on each mail
	 * kind's own screen: eleven kinds send on the shared domain and only
	 * the Client portal invite ever showed a bounce anywhere, so the ten
	 * others failed invisibly. A suppression is keyed by address, and this
	 * screen is keyed the same way.
	 *
	 * Two causes, one of them final. A bounce is a fact about an address
	 * -- mistyped, closed, full -- so Staff fix or confirm it and lift the
	 * block. A complaint is a person saying they did not want the mail;
	 * every Practice shares one sending domain (ADR-0011), so a second
	 * complaint spends everyone's reputation. Such a row still shows,
	 * because "why did the mail stop?" is the question this screen exists
	 * to answer, but it carries no action and says plainly that it stays.
	 * The endpoint refuses one regardless of what this screen offers.
	 *
	 * Lifting a block is consequential and reversible only by another
	 * bounce, so it goes through `ConfirmDialog` -- this repo's one
	 * confirmation mechanism (#473), and GOV.UK's confirm-and-act shape
	 * (docs/design/govuk-alignment.md, ADR-0021) -- rather than firing on
	 * the first click.
	 *
	 * No role read of its own. The Settings hub already hides the link
	 * from anyone but an Owner or an Admin, and the list endpoint's own
	 * refusal is the right sentence for somebody who typed the URL --
	 * which `ListPage`'s `loadError` region then announces.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatInstant } from '#lib/dates.js';
	import {
		clearEmailSuppression,
		loadEmailSuppressions,
		type EmailSuppression
	} from '#lib/emailSuppression.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';

	let suppressions = $state<EmailSuppression[]>([]);
	let isLoaded = $state(false);
	let loadError = $state('');

	// Page-level, not per row: a lifted block takes its own row off the
	// list, so a notice living on that row would have nowhere left to
	// render. A failure leaves the row up, which is why the error half
	// below stays per row, beside the button that earned it.
	let unblockedAddress = $state('');
	let unblockError = $state<Record<string, string>>({});
	let confirmAddress = $state('');

	const practiceId = $derived(page.params.practiceId!);

	onMount(load);

	/*
	 * Deliberately does not put `isLoaded` back to false. It also runs
	 * after a successful unblock, and dropping the whole table back to a
	 * skeleton there would take the success notice down with it for as
	 * long as the read takes.
	 */
	async function load() {
		loadError = '';
		try {
			suppressions = await loadEmailSuppressions(apiFetchWithSession, practiceId);
		} catch (error_) {
			loadError =
				error_ instanceof Error ? error_.message : 'Failed to load the blocked addresses';
		} finally {
			isLoaded = true;
		}
	}

	// Throws nothing on: ConfirmDialog closes on a resolved promise, and
	// a refusal belongs on the row rather than in the dialog.
	async function handleUnblock(address: string) {
		unblockError[address] = '';
		unblockedAddress = '';
		try {
			await clearEmailSuppression(apiFetchWithSession, practiceId, address);
			unblockedAddress = address;
			await load();
		} catch (error_) {
			unblockError[address] =
				error_ instanceof Error ? error_.message : 'Failed to unblock this address';
		}
	}

	// The stored cause, worded as what actually happened. A bounce is
	// about the address; a complaint is about what the person who owns it
	// did. An unrecognized cause prints itself rather than being dropped:
	// a row nobody can explain is still a row that explains why the mail
	// stopped.
	function causeText(suppression: EmailSuppression): string {
		if (suppression.cause === 'bounce') return 'The email could not be delivered';
		if (suppression.cause === 'complaint') return 'The recipient marked the email as spam';
		return suppression.cause;
	}

	const columns = [
		{ label: 'Address', accessor: (row: EmailSuppression) => row.address },
		{ label: 'Why it stopped', accessor: causeText },
		{
			label: 'Blocked since',
			accessor: (row: EmailSuppression) => formatInstant(row.createdAt),
			// ADR-0022: the display string is a day, the exact instant stays
			// underneath as the <time> element's machine-readable value.
			datetimeAccessor: (row: EmailSuppression) => row.createdAt,
			variant: 'meta' as const
		}
	];
</script>

{#snippet intro()}
	<Text
		text="Doula Cloud stops writing to an address once an email to it comes back undelivered, or once the recipient marks an email as spam. Nothing this Practice sends reaches a blocked address until the block is lifted."
	/>
{/snippet}

{#snippet rowActions(suppression: EmailSuppression)}
	{#if suppression.clearable}
		<!--
			The button names the address it is about only to a screen
			reader: sighted readers have the row itself, and repeating the
			address in every label would be a column of noise at 320px. The
			same describedBy/visually-hidden pair the Staff roster's own
			per-row actions use.
		-->
		<!--
			Secondary and small, like every other per-row action in the app
			(the Staff roster's own five): a table of bounces would
			otherwise carry one full-size primary button per row, and the
			emphasis belongs on the confirmation step rather than on the
			row.
		-->
		<Button
			label="Unblock"
			variant="secondary"
			size="sm"
			describedBy="{suppression.address}-unblock-name"
			onClick={() => (confirmAddress = suppression.address)}
		/>
		<span class="visually-hidden" id="{suppression.address}-unblock-name">
			{suppression.address}
		</span>
		<ConfirmDialog
			bind:open={
				() => confirmAddress === suppression.address,
				(value) => {
					if (!value) confirmAddress = '';
				}
			}
			title="Unblock this address"
			consequence={`Doula Cloud writes to ${suppression.address} again. If an email to it comes back undelivered, it is blocked again.`}
			confirmLabel="Unblock this address"
			onConfirm={() => handleUnblock(suppression.address)}
		/>
	{:else}
		<!--
			ADR-0029: a complaint is never lifted, so there is no button to
			hide -- only a sentence saying so, in the cell where the button
			would otherwise be.
		-->
		<Text text="This block stays. It cannot be undone." step="body-sm" tone="variant" />
	{/if}
	{#if unblockError[suppression.address]}
		<Notice variant="error" message={unblockError[suppression.address]} />
	{/if}
{/snippet}

{#snippet content()}
	{#if unblockedAddress}
		<Notice variant="status" message="{unblockedAddress} can receive email again." />
	{/if}

	{#if suppressions.length === 0}
		<!--
			Most Practices are here, and the sentence is written for them: a
			table header over the word "none" answers nothing, so the empty
			state says what the empty list means instead.
		-->
		<Text
			text="No blocked addresses. Every address this Practice writes to can still receive its email."
		/>
	{:else}
		<!--
			hasMore is always false: the endpoint answers one Practice's
			blocked addresses whole. It is a bounded population -- a
			Practice with enough of these to page through has a mail
			problem no list would fix.
		-->
		<DataTable
			{columns}
			rows={suppressions}
			rowActions={{ label: 'Actions', content: rowActions }}
			hasMore={false}
			emptyMessage="No blocked addresses."
		/>
	{/if}
{/snippet}

<ListPage
	title="Blocked email addresses"
	{intro}
	{content}
	loading={isLoaded ? undefined : 'Loading the blocked addresses'}
	loadError={loadError || undefined}
/>
