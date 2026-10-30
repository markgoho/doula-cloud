<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { clientsResultSummary, loadClients, type ClientListItem, type OpenEngagement } from '#lib/client.js';
	import type { ComponentProps } from 'svelte';
	import { kindLabel } from '#lib/engagementRequest.js';
	import { engagementStatusText } from '#lib/engagementDetail.js';
	import { startHref } from './new/intake.js';
	import { formatAmount, invoiceRowStatusLabel, invoiceRowStatusVariant, invoiceStatusLabel } from '#lib/invoice.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	// #539 (ADR-0017): +page.ts's load already decided, before this
	// component mounted, whether the caller is a contractor Doula with
	// neither the owner nor the admin role. `data` is optional only so
	// this component's own spec (which renders it directly, bypassing
	// SvelteKit's load cycle) keeps working without a fixture -- every
	// real navigation always supplies it.
	let { data }: { data?: PageProperties['data'] } = $props();
	const isContractor = $derived(data?.isContractor ?? false);
	// #1609: whether the Practice holds any Client record, which decides
	// where the header link goes. Defaults to the search, which is right
	// at a Practice of any size. Undefined while +page.ts's read runs
	// (#2249).
	let hasAnyClient = $state<boolean>();
	$effect(() => {
		const gate = data?.hasAnyClient ?? Promise.resolve(true);
		let isCurrent = true;
		void gate.then((answer) => {
			if (isCurrent) hasAnyClient = answer;
		});
		return () => {
			isCurrent = false;
		};
	});

	// Starts empty: unlike Billing and Invoices this screen fetches its own
	// first page in the effect below rather than through a load, so every
	// page it holds arrives through this list.
	const clients = new PaginatedList<ClientListItem>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) =>
			loadClients(apiFetchWithSession, page.params.practiceId!, {
				cursor,
				showAll: isShowingEveryone
			}),
		failureMessage: 'Failed to load more Clients'
	});
	let error = $state('');
	let isLoaded = $state(false);
	const isReady = $derived(isLoaded && hasAnyClient !== undefined);

	// The default filter is "Clients with work" (ADR-0017); `?all=true`
	// switches to everyone -- including a Client whose only Request was
	// refused. A URL param, not local state: shareable, survives a
	// reload, and works with the back button, all for free from
	// SvelteKit's own navigation (#499).
	let isShowingEveryone = $derived(page.url.searchParams.get('all') === 'true');

	function clientsPath(isShowingEveryone: boolean): string {
		const base = resolve('/practices/[practiceId]/clients', {
			practiceId: page.params.practiceId!
		});
		return isShowingEveryone ? `${base}?all=true` : base;
	}

	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property Checkbox's onChange wraps 1:1 (same exception Checkbox.svelte itself carries)
	async function handleToggleAll(checked: boolean) {
		// Before the navigation, so a "Load more" already in flight is
		// abandoned rather than merged into the new filter's list. The rows
		// stay up until the new filter's first page arrives.
		clients.abandon();
		await goto(clientsPath(checked));
	}

	const summary = $derived(clientsResultSummary(clients.items.length, clients.hasMore, isShowingEveryone));

	// ADR-0017: a pending Engagement Request shows on its Client's row.
	// She can hold one of each kind at once, hence the join rather than a
	// single value.
	function pendingRequestText(client: ClientListItem): string {
		const kinds = client.pendingRequestKinds ?? [];
		if (kinds.length === 0) return '';
		const labels = kinds.map((kind) => (kind === 'birth' ? 'Birth' : 'Postpartum'));
		return `${labels.join(' & ')} request pending`;
	}

	// #458: the default view is always "Clients with work" (ADR-0017), so
	// every row there already has it -- the column would say the same
	// thing on every row. It only carries information once "See everyone"
	// mixes the two together.
	function workStatusText(client: ClientListItem): string {
		return client.hasWork ? 'Has work' : 'No work yet';
	}

	type StatusBadge = { label: string; variant: ComponentProps<typeof Badge>['variant'] };

	// #264 (RA-G6), #2022: the Contract lifecycle as a badge. Signed is
	// done, sent waits on the Client, and the rest are facts with nothing
	// to do.
	const contractBadges: Record<string, StatusBadge> = {
		draft: { label: 'Contract draft', variant: 'neutral' },
		sent: { label: 'Contract sent', variant: 'info' },
		signed: { label: 'Contract signed', variant: 'success' },
		voided: { label: 'Contract voided', variant: 'neutral' }
	};

	const NO_CONTRACT_BADGE: StatusBadge = { label: 'No Contract yet', variant: 'neutral' };

	// #2022: one open Engagement's status as badges -- its care phase, its
	// Contract, and its money wherever the BFF's role gate let the field
	// through the wire at all (ADR-0006/ADR-0008). A field this Reader may
	// not see is absent from `line` entirely, not blanked, so there is
	// nothing here to hide -- only to append when present.
	//
	// The badges replace #264's sentence ("Contract: Signed · Doula: … ·
	// Active · Outstanding ($1,500.00)"), whose one long line set this
	// list's floor at 101.75rem, past what a desktop window holds. The
	// Doula's name is on the Engagement's own page, one link away.
	//
	// #741: `invoiceStatus`/`invoiceAmountCents` are the BFF's answer to
	// "is anything outstanding under this Contract", not merely its newest
	// Invoice's status (list.go's OpenEngagement doc comment). So an `open`
	// result reads "Owes", never as the latest of several Invoices, and it
	// is the one badge that asks for something. "Owes", not #741's
	// "Outstanding": a typical row (Postpartum, Active, Contract signed and
	// the money) wrapped at 1440px in a cell held to the measure, and the
	// shorter word keeps it on one line. A doula says it that way too.
	function lineBadges(line: OpenEngagement): StatusBadge[] {
		const badges: StatusBadge[] = [
			// #1512: the care phase in the word every Staff screen shows.
			{ label: engagementStatusText(line.engagementStatus), variant: 'neutral' },
			(line.contractStatus && contractBadges[line.contractStatus]) || NO_CONTRACT_BADGE
		];
		if (line.invoiceStatus) {
			const amount = line.invoiceAmountCents === undefined ? '' : ` ${formatAmount(line.invoiceAmountCents)}`;
			// #2130: a late Invoice reads Overdue, from the BFF's one count
			// and through the label pair every Staff screen reads, so it
			// says here what it says on its Engagement and on Invoices.
			const standing = { status: line.invoiceStatus, daysOverdue: line.daysOverdue ?? 0 };
			if (standing.daysOverdue > 0) {
				badges.push({ label: `${invoiceRowStatusLabel(standing)}${amount}`, variant: invoiceRowStatusVariant(standing) });
			} else if (line.invoiceStatus === 'open') {
				badges.push({ label: `Owes${amount}`, variant: 'warning' });
			} else if (line.invoiceStatus === 'paid') {
				badges.push({ label: `Paid${amount}`, variant: 'success' });
			} else {
				badges.push({ label: `Invoice ${invoiceStatusLabel(line.invoiceStatus).toLowerCase()}${amount}`, variant: 'neutral' });
			}
		}
		if (line.feeCents !== undefined) {
			badges.push({ label: `Your fee ${formatAmount(line.feeCents)}`, variant: 'neutral' });
		}
		return badges;
	}

	// The Status cell's words. They name the cell to `DataTable`, and they
	// decide whether a record card keeps the row at all (#2022): one clause
	// per open Engagement, then the pending Request.
	function statusText(client: ClientListItem): string {
		const lines = (client.openEngagements ?? []).map(
			(line) => `${kindLabel(line.kind)}: ${lineBadges(line).map((badge) => badge.label).join(', ')}`
		);
		const pending = pendingRequestText(client);
		return [...lines, ...(pending ? [pending] : [])].join('; ');
	}

	// #2022: Email and Portal invite are on the Client hub, where the
	// Blocked email addresses link (#785) went with them, and a pending
	// Request is a badge in Status. What is left fits a desktop.
	const columns = $derived([
		{ label: 'Name', accessor: (client: ClientListItem) => client.name },
		{ label: 'Status', accessor: statusText, content: statusCell },
		...(isShowingEveryone ? [{ label: 'Work', accessor: workStatusText }] : [])
	]);

	// The Client detail hub (#494) -- DataTable links only its first
	// column, per rowHref's own contract.
	function clientHref(client: ClientListItem): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId: client.clientId
		});
	}

	// #1511: the Engagement page a rollup line's kind opens.
	function engagementHref(line: OpenEngagement): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: line.engagementId
		});
	}

	// Reloads page one against the current filter -- on first mount, on a
	// toggle click, and on a browser back/forward that lands on a
	// different `all` value, since `isShowingEveryone` is derived from
	// `page.url` and every one of those re-runs this effect.
	$effect(() => {
		void loadFirstPage(isShowingEveryone);
	});

	/*
	 * "Load more" is the only fetch that *merges* into what is on screen,
	 * so it is the only one a stale response can corrupt: a page-two
	 * request already in flight when the reader flips the toggle would
	 * append the old filter's Clients onto the new filter's list, and
	 * nothing afterwards would take them back out. `clients.reset` below
	 * abandons any such response -- the guard now lives in PaginatedList,
	 * where every paging list gets it rather than only this one.
	 *
	 * loadFirstPage needs no guard of its own. It *replaces* the list
	 * wholesale rather than appending, so two of them racing can only
	 * leave a whole, self-consistent result set from a filter one flip out
	 * of date -- which the effect's next run corrects -- never a list
	 * mixing both.
	 */
	async function loadFirstPage(isShowingEveryone: boolean) {
		isLoaded = false;
		error = '';
		try {
			clients.reset(
				await loadClients(apiFetchWithSession, page.params.practiceId!, {
					showAll: isShowingEveryone
				})
			);
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Clients';
		}
	}

	function searchHref(): string {
		return resolve('/practices/[practiceId]/clients/search', {
			practiceId: page.params.practiceId!
		});
	}

	// #539 (ADR-0017): "No Clients yet." is silent about *why* to a
	// contractor Doula, who never gets to make one -- the same screen
	// tells her plainly that work reaches her as an Offer instead.
	//
	// #1706: the default filter is "Clients with work" (ADR-0017), so an
	// empty default view at a Practice that holds a Client record (the
	// `all=true` count +page.ts read) means nobody has work yet, not that
	// nobody exists -- and "No Clients yet." would contradict the "Find or
	// add a Client" link above it. With "See everyone" on there is no
	// filter left to point at, so the plain message stays.
	const emptyMessage = $derived.by(() => {
		if (isContractor) return 'Work reaches you as an Offer, so there are no Clients here yet.';
		if (hasAnyClient && !isShowingEveryone) {
			return 'No Client has work yet. Turn on See everyone to list every Client.';
		}
		return 'No Clients yet.';
	});
</script>

{#snippet statusCell(client: ClientListItem, view: DataTableView)}
	<!--
		#264, #2022: one <li> per open Engagement -- ADR-0017's "a Client can
		hold more than one concurrent open Engagement" is why this is a list,
		so a second (or third) line is never dropped -- then the pending
		Request, which a Client can hold before she has any Engagement.
		Nothing renders for a Client with neither, and `statusText` is empty
		for her too, so her record card leaves the row out.
	-->
	{@const pending = pendingRequestText(client)}
	{#if (client.openEngagements ?? []).length > 0 || pending}
		<!--
			#1511: each line's kind opens the Engagement's page. A new
			Engagement has no Visit, Contract or Invoice, so no other screen
			links to it yet. The link's description names the Client, so a
			list of links read out of the table still tells one Client's
			"Birth" from the next. The name is `hidden`: a description can
			read hidden text, and the cell's own name then stays the line.
			DataTable draws this cell once per view, hence the view in the id.
		-->
		<span id="status-{view}-{client.clientId}" hidden>{client.name}</span>
		<ul class="status-list">
			{#each client.openEngagements ?? [] as line (line.engagementId)}
				<li>
					<cluster-l space="var(--space-related)" align="center">
						<Link
							href={engagementHref(line)}
							label={kindLabel(line.kind)}
							describedBy="status-{view}-{client.clientId}"
						/>
						{#each lineBadges(line) as badge (badge.label)}
							<Badge label={badge.label} variant={badge.variant} />
						{/each}
					</cluster-l>
				</li>
			{/each}
			{#if pending}
				<!-- ADR-0017: a pending Request waits on the Practice, so it is
				     the warning. -->
				<li><Badge label={pending} variant="warning" /></li>
			{/if}
		</ul>
	{/if}
{/snippet}

{#snippet actions()}
	<!--
		Once the Practice holds a Client record, this link lands on the
		search (#498, #539, ADR-0017), not on intake directly, so a
		returning Client is found rather than typed a second time. A miss
		there hands the reader on to intake. Named for both errands it
		serves -- finding a returning Client is the one a contractor
		Doula's own attached-Clients list already covers, which is why she
		does not see this at all (#539).

		While the Practice holds no Client record there is nobody to find,
		so it opens the name question and says only what it does (#1609,
		ADR-0017's amendment of 2026-10-02). +page.ts decides which, from
		the count of records, not from the filtered rows below.
	-->
	{#if hasAnyClient === undefined}
		<Skeleton variant="text" lines={1} />
	{:else if hasAnyClient}
		<Link href={searchHref()} label="Find or add a Client" />
	{:else}
		<Link href={startHref(page.params.practiceId!, 'clients')} label="Add a Client" />
	{/if}
{/snippet}

{#snippet content()}
	<!--
		The default view is "Clients with work" (ADR-0017); this switches to
		everyone, including a Client whose only Request was refused -- nothing
		new is built for her, she is simply no longer filtered out.

		#2068: the toggle is the list's filter, so it sits in the list's
		filter bar, directly above the list it narrows (Law of Proximity).
		A switch acts at once, so the bar draws no Apply; the toggle's own
		change navigates.
	-->
	<FilterBar
		action={clientsPath(false)}
		legend="Choose who to show"
		status={isReady ? summary : ''}
		empty={isReady && clients.items.length === 0}
		isAppliedOnChange
	>
		<LabeledField label="See everyone" orientation="inline">
			{#snippet children(control)}
				<Checkbox variant="toggle" checked={isShowingEveryone} onChange={handleToggleAll} {...control} />
			{/snippet}
		</LabeledField>
	</FilterBar>

	{#if error}
		<Notice message={error} variant="error" />
	{:else if isReady}
		<!--
			No floor of its own (#2022): with Email, Portal invite and the
			Engagements sentence gone, the typical pilot screen's table fits
			`DataTable`'s own 55rem, so the 101.75rem this route used to set
			(#1945, #1981) went with them. #2130 put an "Overdue $1,500.00"
			badge on a typical row and measured again on 2026-10-26: the
			typical table needs 782px on macOS/Chromium with it and without
			it, under the 880px (55rem) floor, so the floor did not move.

			#539, #501 (ADR-0017): hiding "Find or add a Client" above took
			away this door's only link for a contractor Doula, so it comes
			back as the empty list's action (#2015) -- once she has an
			attached Client, the narrowed list is already her route to it.
			Everyone else has "Find or add a Client" above the list.
		-->
		<DataTable
			{columns}
			rows={clients.items}
			rowHref={clientHref}
			hasMore={clients.hasMore}
			onLoadMore={() => clients.loadMore()}
			isLoadingMore={clients.isLoadingMore}
			loadMoreError={clients.loadMoreError}
			{emptyMessage}
			emptyAction={isContractor
				? { label: 'How to add Clients of your own', href: searchHref() }
				: undefined}
		/>
	{:else}
		<Skeleton variant="row" lines={8} label="Loading Clients" />
	{/if}
{/snippet}

<ListPage title="Clients" actions={isContractor ? undefined : actions} {content} />

<style>
	@layer components {
		/*
		 * The Status cell's own list (#264, #2022). The markup is written in
		 * this file, so this file's scope reaches it and DataTable is left
		 * owning the CELL rather than what a caller puts in it (#740). The
		 * cell's vertical room is DataTable's own (`td.content`).
		 */
		.status-list {
			display: grid;
			gap: var(--space-1);
			margin: 0;
		}
	}
</style>
