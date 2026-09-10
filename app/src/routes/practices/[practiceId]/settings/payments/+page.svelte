<script lang="ts">
	/*
	 * Where a Practice Owner meets Stripe's hosted onboarding (#442).
	 *
	 * Two things this screen owes her, both found by walking that flow in
	 * the Sandbox rather than guessed at.
	 *
	 * She is told what is coming. The flow asks for a Stripe login with
	 * mandatory two-step authentication, her legal name, date of birth,
	 * home address and the last four digits of her Social Security number,
	 * her bank routing and account numbers, and a support phone and
	 * address. Fifteen minutes, a phone in her hand, and her bank details
	 * are not things to discover halfway through.
	 *
	 * And she is not sent in without a website. Stripe's website field
	 * accepts empty: she clicks Continue with no error, completes every
	 * remaining step, submits, and returns here "done" with card_payments
	 * restricted and nothing on screen saying why (#421). That is the worst
	 * outcome the flow can produce. The button is unavailable until she has
	 * answered #440's question, and PostConnectHandler refuses to mint an
	 * Account Link in that state whatever this screen does.
	 */
	import { onDestroy, onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwner, isOwnerOrAdmin } from '#lib/roles.js';
	import {
		loadConnectStatus,
		connect,
		canConnectStatusStillMove,
		pollConnectStatus,
		nudgeOwnersToConnect,
		CONNECT_STATUS_CHECK_FAILED_MESSAGE,
		CONNECT_NUDGE_SENT_MESSAGE,
		CONNECT_OWNERS_ALREADY_EMAILED_MESSAGE,
		type ConnectStatus,
		type ConnectStatusResult,
		type ConnectStatusPollHandle
	} from '#lib/payments.js';
	import { loadWebsite, type PracticeWebsite } from '#lib/website.js';
	import {
		loadBillingMode,
		setBillingMode,
		loadPaymentTerms,
		setPaymentTerms,
		MIN_PAYMENT_TERMS_DAYS,
		MAX_PAYMENT_TERMS_DAYS,
		netDaysOutOfRangeMessage,
		type BillingMode,
		type PaymentTerms
	} from '#lib/invoice.js';
	import type { PracticeSession } from '../../+layout.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	let status = $state<ConnectStatusResult | undefined>();
	let website = $state<PracticeWebsite | undefined>();
	let error = $state('');
	// The connect button's enabled state mirrors the "owner"-role gating
	// the billing page already uses -- server-side enforcement
	// (RequireOwner) is what actually matters, this is UX only. Resolved
	// once by practices/[practiceId]/+layout.ts (#835), not a fetch here.
	const session = $derived((page.data as { session: PracticeSession }).session);
	let isPracticeOwner = $derived(isOwner(session));
	// Reading the status is one notch wider than starting onboarding:
	// ADR-0008's Stripe Connect state row is Owner and Admin (#267). This
	// decides whether the screen asks the endpoint at all, so it mirrors
	// `staffauth.OwnerAndAdmin` on the route exactly -- the BFF is what
	// refuses, this only decides what to ask for and what to show.
	let isPracticeOwnerOrAdmin = $derived(isOwnerOrAdmin(session));
	let connectParameter = $derived(page.url.searchParams.get('connect'));

	let connectError = $state('');
	let isConnecting = $state(false);

	// #259. Coming back from Stripe is a full document load, so the one
	// fetch in onMount below is already fresh -- but it can land before
	// Stripe's own Account object reflects the form just submitted, which
	// is exactly what MO-G11 watched happen. There is no navigation event
	// to hook a re-read to once she is already sitting on this page (an
	// external redirect, not a client-side one), so the fix is a short
	// poll rather than an invalidate -- reasoning captured in #259's own
	// re-verified triage comment. `refreshError` never clears `status`: a
	// failed re-read keeps the last good answer on screen rather than
	// blanking it or routing through FormPage's `loadError`, which would
	// replace the whole page over a check that merely didn't go through.
	let refreshError = $state('');
	let isCheckingStatus = $state(false);
	let isPolling = $state(false);
	let pollHandle: ConnectStatusPollHandle | undefined;
	// Plain, not $state: only onMount's own async chain reads it, to
	// decide whether it is still allowed to start a poll after leaving
	// the two awaits below. A visit that leaves fast enough to destroy
	// this screen before either fetch resolves must not start a poll
	// that nothing will ever be left to stop.
	let isDestroyed = false;

	// #271: read by any Staff member (#270's own reasoning for the
	// sibling "can this Practice raise an Invoice at all" fact), so this
	// runs unconditionally rather than behind the Owner/Admin guard the
	// Connect status fetch below uses. billingMode undefined is
	// ambiguous on its own -- still loading, or loaded and never
	// chosen -- so isBillingModeLoaded tells the two apart.
	let billingMode = $state<BillingMode | undefined>();
	let isBillingModeLoaded = $state(false);
	let billingModeLoadError = $state('');
	let chosenBillingMode = $state<BillingMode>('stripe');
	let isSavingBillingMode = $state(false);
	let billingModeSaveError = $state('');

	onMount(async () => {
		try {
			billingMode = await loadBillingMode(apiFetchWithSession, page.params.practiceId!);
			chosenBillingMode = billingMode ?? 'stripe';
			isBillingModeLoaded = true;
		} catch (error_) {
			billingModeLoadError = error_ instanceof Error ? error_.message : 'Failed to load billing mode';
		}
	});

	/*
	 * #768: how many days after an Invoice is raised it falls due, and so
	 * what this Practice's own book calls late. Every Staff member reads
	 * it; an Owner or an Admin sets it. A Practice that has never set one
	 * runs on 30 days, and the screen says so rather than showing an
	 * empty field a reader has to interpret.
	 */
	let paymentTerms = $state<PaymentTerms | undefined>();
	let paymentTermsLoadError = $state('');
	let typedNetDays = $state('');
	let isSavingPaymentTerms = $state(false);
	let paymentTermsError = $state('');

	/*
	 * GOV.UK's Recover from validation errors pattern (ADR-0021,
	 * docs/design/govuk-alignment.md): a refused value is said twice --
	 * once at the top of the page, where ErrorSummary takes focus and
	 * announces it, and once beside the field itself. This is the first
	 * field on this screen a person can get wrong, so the summary is
	 * earned here and was not before. The id is fixed rather than
	 * generated so the summary's own `<a href="#id">` can reach the input
	 * -- an anchor, not a click handler, per the Rule of Least Power.
	 */
	const netDaysFieldId = 'payment-terms-net-days';
	const paymentTermsErrors = $derived(
		paymentTermsError ? [{ message: paymentTermsError, targetId: netDaysFieldId }] : []
	);

	onMount(async () => {
		try {
			paymentTerms = await loadPaymentTerms(apiFetchWithSession, page.params.practiceId!);
			typedNetDays = String(paymentTerms.netDays);
		} catch (error_) {
			paymentTermsLoadError = error_ instanceof Error ? error_.message : 'Failed to load payment terms';
		}
	});

	async function handleSavePaymentTerms() {
		// Refused client-side before it is refused at the boundary, so the
		// reader is told what is wrong in the words of the field rather
		// than in the words of an API. The BFF refuses the same range.
		const netDays = Number(typedNetDays);
		if (
			!Number.isSafeInteger(netDays) ||
			netDays < MIN_PAYMENT_TERMS_DAYS ||
			netDays > MAX_PAYMENT_TERMS_DAYS
		) {
			paymentTermsError = netDaysOutOfRangeMessage;
			return;
		}
		isSavingPaymentTerms = true;
		paymentTermsError = '';
		try {
			paymentTerms = await setPaymentTerms(apiFetchWithSession, page.params.practiceId!, netDays);
			typedNetDays = String(paymentTerms.netDays);
		} catch (error_) {
			paymentTermsError = error_ instanceof Error ? error_.message : 'Failed to save payment terms';
		} finally {
			isSavingPaymentTerms = false;
		}
	}

	// Owner-only (#271, by analogy to Connect onboarding) -- changing an
	// already-established mode. The initial "ask once" instead rides
	// InvoiceSection's own first-Invoice request, never this screen.
	async function handleChangeBillingMode() {
		isSavingBillingMode = true;
		billingModeSaveError = '';
		try {
			billingMode = await setBillingMode(apiFetchWithSession, page.params.practiceId!, chosenBillingMode);
		} catch (error_) {
			billingModeSaveError = error_ instanceof Error ? error_.message : 'Failed to change billing mode';
		} finally {
			isSavingBillingMode = false;
		}
	}

	onMount(async () => {
		// A Doula is never asked (#267), the same guard the MFA settings
		// screen already uses one notch narrower. Firing the request anyway
		// would earn a 403 and paint the shared refusal string into this
		// screen's load-error region -- a raw permission message where a
		// plain sentence belongs. The website read goes with it: it is here
		// only to decide whether the Connect button may be offered, and on
		// this branch there is no button.
		if (!isPracticeOwnerOrAdmin) return;
		try {
			status = await loadConnectStatus(apiFetchWithSession, page.params.practiceId!);
			website = await loadWebsite(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Stripe Connect status';
			return;
		}
		if (isDestroyed) return;
		// Only the return trip starts the poll, and only while the status
		// can still move on its own -- a settled status, or arriving
		// without `connect=return` at all, must never trigger an extra
		// read (verified by the "no extra read" spec below).
		if (connectParameter === 'return' && canConnectStatusStillMove(status)) {
			isPolling = true;
			pollHandle = pollConnectStatus(apiFetchWithSession, page.params.practiceId!, status, {
				onResult: (result) => {
					status = result;
					refreshError = '';
				},
				onError: () => {
					refreshError = CONNECT_STATUS_CHECK_FAILED_MESSAGE;
				},
				onStopped: () => {
					isPolling = false;
				}
			});
		}
	});

	// Stops a running poll the moment this screen is left, so nothing
	// keeps reading Connect status for a page nobody is looking at --
	// and marks `isDestroyed` for onMount's own chain above, in case this
	// runs before that chain has even started its poll.
	onDestroy(() => {
		isDestroyed = true;
		pollHandle?.stop();
	});

	/* The on-demand half of #259: a visible, focusable control that reads
	   status again right now, independent of whatever the poll above is
	   doing. It exists so nothing depends on a timer she cannot see --
	   the poll is a small, bounded convenience, this is the fallback that
	   always works, including after the poll's ceiling is reached. */
	async function handleCheckStatus() {
		refreshError = '';
		isCheckingStatus = true;
		try {
			status = await loadConnectStatus(apiFetchWithSession, page.params.practiceId!);
		} catch {
			refreshError = CONNECT_STATUS_CHECK_FAILED_MESSAGE;
		} finally {
			isCheckingStatus = false;
		}
	}

	/*
	 * #917 (ADR-0035). The one status where nothing else reaches an Owner
	 * at all: with no Stripe account there is no webhook, so #343's
	 * payout notification can never fire, and ADR-0033 rules out a
	 * sweep. So a reader who can see the gap and cannot close it gets one
	 * control, and pressing it queues a Notification to every Owner.
	 *
	 * `wasNudgeSent` never reverts. The bound lives on the server -- one ask
	 * per Practice per week -- and hiding the control after a press is
	 * the client-side half of it (per "block over warn": prevent here,
	 * enforce at the boundary). A refusal that comes back anyway is
	 * rendered as its own sentence rather than swallowed, because the
	 * ordinary way to meet it is a colleague having asked yesterday from
	 * her own screen, which this one cannot see.
	 */
	let isNudging = $state(false);
	let wasNudgeSent = $state(false);
	let nudgeError = $state('');

	async function handleNudgeOwners() {
		nudgeError = '';
		isNudging = true;
		try {
			await nudgeOwnersToConnect(apiFetchWithSession, page.params.practiceId!);
			wasNudgeSent = true;
		} catch (error_) {
			nudgeError = error_ instanceof Error ? error_.message : 'Failed to email the Practice Owners';
		} finally {
			isNudging = false;
		}
	}

	async function handleConnect() {
		connectError = '';
		isConnecting = true;
		try {
			const onboardingUrl = await connect(apiFetchWithSession, page.params.practiceId!);
			location.assign(onboardingUrl);
		} catch (error_) {
			connectError = error_ instanceof Error ? error_.message : 'Failed to start Stripe Connect onboarding';
		} finally {
			isConnecting = false;
		}
	}

	// One row per status rather than four parallel maps: the label, the
	// badge, what it means for the Practice in the Owner's words, and
	// whether reopening Stripe's hosted form could help.
	//
	// `onboarding` is not derivable from the status alone. `pending` means
	// Stripe is reviewing and there is nothing to supply, so offering the
	// button would be a dead end -- but `payouts_restricted` can mean
	// either (Stripe reviewing the bank details, or the Owner never
	// entered any), so that one asks whether anything is outstanding.
	const statusCopy: Record<
		ConnectStatus,
		{
			label: string;
			variant: 'neutral' | 'warning' | 'success';
			explanation: string;
			onboarding: 'always' | 'never' | 'if-outstanding';
		}
	> = {
		not_connected: {
			label: 'Not connected',
			variant: 'neutral',
			explanation: 'Clients cannot pay their invoices yet.',
			onboarding: 'always'
		},
		onboarding_incomplete: {
			label: 'Onboarding incomplete',
			variant: 'warning',
			explanation: 'Stripe still needs some details before Clients can pay this Practice.',
			onboarding: 'always'
		},
		pending: {
			label: 'Awaiting Stripe review',
			variant: 'warning',
			explanation: 'Stripe is reviewing the details already submitted. Nothing more is needed right now.',
			onboarding: 'never'
		},
		payouts_restricted: {
			label: 'Taking payments, payouts on hold',
			variant: 'warning',
			explanation:
				"Clients can pay their invoices, but Stripe cannot send the money to this Practice's bank yet.",
			onboarding: 'if-outstanding'
		},
		active: {
			label: 'Active',
			variant: 'success',
			explanation: "Clients can pay their invoices, and payouts reach this Practice's bank.",
			onboarding: 'never'
		}
	};

	let copy = $derived(status === undefined ? undefined : statusCopy[status.status]);

	/* How many things Stripe is still waiting on, said to the person
	   reading it. The Owner is the one Stripe will ask, so hers is second
	   person; the Admin is reading the state of the Practice's account,
	   which is not an errand she can run. */
	function requirementsSentence(count: number, isOwner: boolean): string {
		const detail = count === 1 ? '1 more detail' : `${count} more details`;
		return isOwner
			? `Stripe needs ${detail} from you.`
			: `Stripe needs ${detail} from a Practice Owner.`;
	}

	let canStartOnboarding = $derived(
		copy?.onboarding === 'always' ||
			(copy?.onboarding === 'if-outstanding' && (status?.requirementsDue.length ?? 0) > 0)
	);

	/* The first gate. `undeclared` is the shape the website endpoint
	   reports for a Practice with no row, so this is "has she answered?"
	   and nothing more. */
	let hasDeclaredWebsite = $derived(website !== undefined && website.mode !== 'undeclared');

	/* The second gate (#443). She answered, and a probe of the page we
	   publish for her found nothing there -- so the URL Stripe would be
	   given is one that 404s, and #382 established the review of that URL
	   is ongoing with no published SLA. Blocking, not warning, on the
	   same rule as the first gate: an answer that does not resolve is
	   worth no more to Stripe than no answer at all.

	   `pending` deliberately does not block. It is the ordinary couple of
	   minutes between publishing and the deploy finishing, and every
	   Practice passes through it on the way to `live`. */
	let isPageFailed = $derived(website?.pageState === 'failed');

	/*
	 * The Connect button is the third of three mutually exclusive branches
	 * `body` renders when `canStartOnboarding`: blocked on no website,
	 * blocked on a failed page, or -- only once neither block applies --
	 * the checklist with the button under it. `actions` is a separate
	 * region from `body` now that both are `FormPage` fieldsets/actions
	 * rather than one `{#if}` chain, so it needs its own name for exactly
	 * the branch that shows the button, or it would render the button
	 * during the two blocked states too (caught by
	 * payments-settings.svelte.spec.ts).
	 */
	let canConnect = $derived(canStartOnboarding && hasDeclaredWebsite && !isPageFailed);

	let websiteHref = $derived(
		resolve('/practices/[practiceId]/settings/website', { practiceId: page.params.practiceId! })
	);

	/* What Stripe's hosted flow actually asks for, in the order it asks.
	   Walked end to end in the Sandbox on #421 and again on #442 -- not
	   read off a doc page, and not a guess at what a merchant onboarding
	   might want.

	   Two of the things #421 met are missing on purpose, because #442
	   removed them: the industry dropdown (Stripe's list has no doula or
	   birth-work category, so the account is created under one already)
	   and the website field. Neither appears in the walked flow any more,
	   so neither belongs in a list of what she will be asked. */
	const stripeAsksFor = [
		'A Stripe login: an email address, a password, and two-step authentication on your phone. Stripe gives you a backup code — keep it.',
		'Whether your Practice is registered with the government, and whether it has an EIN.',
		'Your legal name, date of birth, home address, phone number, and the last four digits of your Social Security number.',
		'Your bank routing and account numbers, so Stripe can send you the money.',
		'A phone number and postal address Clients can use to reach you about a payment.'
	];

	/* One question the two answers to #440 do not share. A Practice who
	   published a page here wrote a description of what she offers, and it
	   travels to Stripe with the account, so Stripe never asks; a Practice
	   who gave her own address wrote nothing, and Stripe asks her for it
	   in its own words. Confirmed against the Sandbox on two accounts
	   created with the same parameters but that one field (#442) --
	   defaults.profile.product_description stays outstanding on the second
	   and disappears on the first.

	   She is not asked for it here instead. #440 asks a Practice for
	   exactly the facts nobody else can supply and stops, and a box on
	   this screen would be a third place to keep the same sentence. */
	let willAskForProductDescription = $derived(website?.mode === 'own');

	/* Coming back from Stripe is not the same as being finished. Stripe's
	   website field accepts empty, and #421 watched an account come back
	   "done" and restricted; the account is also restricted while Stripe
	   is still reviewing. So the return notice reads the status rather
	   than assuming the trip worked. */
	let isBackFromStripeRestricted = $derived(
		connectParameter === 'return' && status !== undefined && status.status === 'onboarding_incomplete'
	);
</script>

{#snippet billingModeSection()}
	<!--
		#271: a Practice-level choice between Stripe-hosted Invoicing and
		billing by hand -- readable by any Staff member, changeable only by
		an Owner, and never asked here for the first time (that rides
		InvoiceSection's own inline ask on the first Invoice raised).
	-->
	{#if billingModeLoadError}
		<Notice variant="error" message={billingModeLoadError} />
	{:else if !isBillingModeLoaded}
		<Text text="Loading billing mode…" />
	{:else if billingMode === undefined}
		<Text text="Not chosen yet -- this is asked the first time Staff raises an Invoice." />
	{:else}
		<Text
			text={billingMode === 'by_hand'
				? 'This Practice bills Clients by hand.'
				: 'This Practice bills Clients through Stripe.'}
		/>
		{#if isPracticeOwner}
			<RadioGroup
				legend="Change billing mode"
				options={[
					{ value: 'stripe' as const, label: 'Stripe' },
					{ value: 'by_hand' as const, label: 'By hand' }
				]}
				value={chosenBillingMode}
				onChange={(value) => (chosenBillingMode = value)}
			/>
			<Button
				label="Save"
				onClick={handleChangeBillingMode}
				loading={isSavingBillingMode}
				disabled={chosenBillingMode === billingMode}
			/>
			{#if billingModeSaveError}
				<Notice variant="error" message={billingModeSaveError} />
			{/if}
		{/if}
	{/if}
{/snippet}

{#snippet errorSummary()}
	<ErrorSummary errors={paymentTermsErrors} />
{/snippet}

{#snippet paymentTermsSection()}
	<!--
		#768: an Invoice falls due this many days after it is raised, on
		both rails. Changing it never moves an Invoice already raised --
		each one keeps the terms it was billed under -- so the sentence
		below says "the next invoice", not "invoices".
	-->
	{#if paymentTermsLoadError}
		<Notice variant="error" message={paymentTermsLoadError} />
	{:else if paymentTerms === undefined}
		<Text text="Loading payment terms…" />
	{:else}
		<Text
			text={paymentTerms.isDefault
				? 'Invoices are due 30 days after they are raised. That is the default; this Practice has not set terms of its own.'
				: `Invoices are due ${paymentTerms.netDays} days after they are raised.`}
		/>
		{#if isPracticeOwnerOrAdmin}
			<LabeledField
				id={netDaysFieldId}
				label="Days to pay"
				hint="Applies to the next invoice raised. An invoice already raised keeps the terms it was billed under."
				error={paymentTermsError || undefined}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="number"
						min={MIN_PAYMENT_TERMS_DAYS}
						inputmode="numeric"
						value={typedNetDays}
						onInput={(value) => (typedNetDays = value)}
					/>
				{/snippet}
			</LabeledField>
			<Button
				label="Update payment terms"
				onClick={handleSavePaymentTerms}
				loading={isSavingPaymentTerms}
				disabled={typedNetDays === String(paymentTerms.netDays) && !paymentTerms.isDefault}
			/>
		{/if}
	{/if}
{/snippet}

{#snippet intro()}
	<!--
		#256: this screen used to say nothing about itself until the Connect
		status resolved, so a person landing here directly had no way to
		tell it apart from Credits (Doula Cloud's own billing to the
		Practice) without opening both -- two different counterparties
		hidden behind two similar-sounding names. FormPage now renders
		`intro` in the loading and loadError branches too (#256), so this
		sentence is on screen before the Stripe fetch resolves, not after.
		"Stripe account" itself is on CONTEXT.md's Connected-account _Avoid_
		list, so this says "connects Stripe", matching the button copy.
	-->
	<Text text="This is where a Practice connects Stripe, so its Clients can pay it directly." />
	<Text text="Credits is a separate screen, where this Practice buys Credits from Doula Cloud." />
{/snippet}

{#snippet body()}
	{#if !isPracticeOwnerOrAdmin}
		<!--
			A plain sentence about whose screen this is, never the BFF's own
			refusal wording (#267). She is told what the screen is for and
			who holds it, which is the whole of what there is to say -- the
			status itself is not hers to see, so there is nothing further
			here and no button to explain the absence of.
		-->
		<Notice
			variant="status"
			message="Only a Practice Owner or Admin can see how this Practice gets paid."
		/>
	{:else}
		<!--
			A non-null assertion, not another `{#if status}`: `body` is one of
			`FormPage`'s `fieldsets`, which only render once neither `loadError`
			nor `loading` is active below, and `loading` is exactly `status ===
			undefined` for an Owner or Admin -- so `status` is always defined by
			the time this branch runs. A second guard here would compile to a
			branch that never takes its false path, which the coverage gate
			would then refuse.
		-->
		<!--
			aria-live="polite" on both regions below (#259): a re-read, from
			the poll or from the on-demand button, replaces status wholesale,
			and the badge, the explanation and the requirement count have to
			be announced as the update they are rather than sit there
			silently changed. Two regions, not one, so the return/refresh
			banner between them -- which carries its own role via Notice --
			is never nested inside another live region.
		-->
		<cluster-l aria-live="polite">
			<Text text="Stripe Connect status:" />
			<Badge label={statusCopy[status!.status].label} variant={statusCopy[status!.status].variant} />
		</cluster-l>

		{#if isBackFromStripeRestricted}
			<Notice
				variant="error"
				message="Stripe still needs something before Clients can pay this Practice. Finish the form below."
			/>
		{:else if connectParameter === 'return' && isPolling}
			<!-- The banner's wording matches what the screen actually does
			     (#259): it only promises to keep checking while a poll is
			     actually running. -->
			<Notice
				variant="status"
				message="Stripe onboarding finished. We're checking with Stripe again shortly, in case its review is still catching up."
			/>
		{:else if connectParameter === 'return'}
			<!-- No poll is running -- the status already settled on the
			     first read, or the poll already reached its ceiling -- so
			     this stops short of promising an update that is not coming. -->
			<Notice variant="status" message="Stripe onboarding finished." />
		{:else if connectParameter === 'refresh'}
			<Notice variant="status" message="The Stripe onboarding link expired. Start again below." />
		{/if}

		<stack-l space="var(--space-2)" aria-live="polite">
			<Text text={statusCopy[status!.status].explanation} />

			<!-- The count, not the list. requirementsDue holds Stripe's own
			machine-readable field paths ("configuration.merchant.mcc"), which
			name nothing an Owner recognizes. The place those get asked in words
			is Stripe's hosted form, which the button below opens; the paths stay
			in the database for the audit trail.

			"from you" is only true of the Owner: she is the one Stripe will ask,
			and PostConnectHandler refuses anybody else. An Admin reads the same
			count as a fact about the Practice's account rather than as an errand
			of her own -- the same reason the branch below tells her who connects
			Stripe instead of handing her the checklist. -->
			{#if status!.requirementsDue.length > 0}
				<Text
					text={requirementsSentence(status!.requirementsDue.length, isPracticeOwner)}
				/>
			{/if}
		</stack-l>

		<!--
			The on-demand half of #259. Always offered, whatever `status` is
			and whatever the poll above is doing -- "at any time" per the
			issue's acceptance criteria -- so nothing about seeing the
			current answer depends on a timer she cannot see or on having
			just come back from Stripe at all.
		-->
		<cluster-l space="var(--space-3)">
			<Button label="Check status again" variant="secondary" size="sm" onClick={handleCheckStatus} loading={isCheckingStatus} />
		</cluster-l>
		{#if refreshError}
			<Notice variant="error" message={refreshError} />
		{/if}

		{#if canStartOnboarding && !isPracticeOwner}
			<!--
				A fact about the Practice and the role that clears it (#270),
				not a refusal aimed at her: PostConnectHandler is Owner-only
				(staffauth.RequireOwner), so she gets the one sentence that is
				true for her instead of a checklist she cannot act on. #267
				gave her the *state* of the rail her Invoices are paid on, not
				the Owner's half-finished errand.
			-->
			<Text text="A Practice Owner has to connect Stripe." />
			<!--
				#917 (ADR-0035). Two different true things, and which one she
				reads turns on whether anything else in the product can reach
				an Owner about this at all.

				`not_connected` is the one status that produces no Stripe
				webhook, so #343's payout Notification can never fire for it
				and ADR-0033 rules out a sweep that would notice it. Nobody
				has been told, and nobody will be, so she is offered the ask.

				Every other status on this branch has an account behind it,
				which means Stripe raised requirements, which means #343
				already mailed every Owner once for this episode. A second
				control there would be a duplicate she has no way to know she
				is sending, so she is told what already happened instead.
			-->
			{#if status!.status === 'not_connected'}
				{#if wasNudgeSent}
					<Notice variant="status" message={CONNECT_NUDGE_SENT_MESSAGE} />
				{:else}
					<Text
						text="Doula Cloud can email every Practice Owner about this. It sends this reminder at most once a week."
					/>
					<cluster-l space="var(--space-3)">
						<Button
							label="Email the Practice Owners"
							variant="secondary"
							onClick={handleNudgeOwners}
							loading={isNudging}
						/>
					</cluster-l>
				{/if}
				{#if nudgeError}
					<Notice variant="error" message={nudgeError} />
				{/if}
			{:else}
				<Notice variant="status" message={CONNECT_OWNERS_ALREADY_EMAILED_MESSAGE} />
			{/if}
		{:else if canStartOnboarding && !hasDeclaredWebsite}
			<!--
				Block, do not warn. A disabled button with a tooltip would leave
				her guessing what unlocks it; this names the missing thing and
				links to where she supplies it. PostConnectHandler refuses the
				request as well, which is what actually holds the line.
			-->
			<Notice
				variant="info"
				message="Stripe will not process Client payments until it can see this Practice online. The website question needs an answer before Stripe can be connected."
			/>
			<Link href={websiteHref} label="Answer the website question" />
		{:else if canStartOnboarding && isPageFailed}
			<!--
				Block again, and name it as our problem rather than hers: she
				wrote the words and we failed to put them anywhere. Same shape
				as the gate above, and PostConnectHandler refuses the request
				too, which is what actually holds the line.
			-->
			<Notice
				variant="error"
				message="The page published for this Practice is not loading, so Stripe would find nothing at its web address. Publish the website again."
			/>
			<Link href={websiteHref} label="Go to website settings" />
		{:else if canStartOnboarding}
			<!-- A heading and a list, with no <section> around them: the
			     heading already puts this in the screen-reader outline, and a
			     landmark that only repeats the heading is one more thing to
			     skip past. -->
			<stack-l space="var(--space-4)">
				<Heading level={2} text="What Stripe will ask you for" />
				<Text
					text="This takes about fifteen minutes. Have your phone and your bank details with you before you start — Stripe does not save a half-finished form for long."
				/>
				<ul>
					{#each stripeAsksFor as item (item)}
						<li>{item}</li>
					{/each}
					{#if willAskForProductDescription}
						<li>A short description of what your Practice offers, for Stripe's own records.</li>
					{/if}
				</ul>
				<!--
					#421 watched Stripe put FACEBOOK.COM/ROCHESTER onto a walked
					account's Clients' card statements, because it derives the
					descriptor from the website URL when it is not told one. It is
					told one now -- the Practice's own name, set when the account is
					created (#442) -- so what she needs to know is not a warning but
					where to change it.
				-->
				<Text
					text="Stripe puts a short version of your Practice's name on your Clients' card statements. It shows you that text near the end, and you can change it there."
				/>
			</stack-l>
		{/if}
	{/if}
{/snippet}

{#snippet actions()}
	<!--
		The Owner's half only. "Ask a Practice Owner to connect Stripe."
		used to live here as the `{:else}`, which meant an Admin saw it in
		exactly one of the four onboarding states and nothing at all in the
		three blocked ones. It now sits in `body`, on `canStartOnboarding`
		alone, so she is told the same thing whatever is holding the Owner
		up -- and it is written once rather than in two regions that could
		disagree.
	-->
	{#if canConnect && isPracticeOwner}
		<Button
			label={status!.status === 'not_connected' ? 'Connect Stripe' : 'Continue Stripe onboarding'}
			onClick={handleConnect}
			loading={isConnecting}
		/>
		{#if connectError}
			<Notice variant="error" message={connectError} />
		{/if}
	{/if}
{/snippet}

<FormPage
	title="Getting paid"
	{intro}
	errorSummary={paymentTermsErrors.length > 0 ? errorSummary : undefined}
	fieldsets={[
		{ legend: 'Billing mode', content: billingModeSection },
		{ legend: 'Payment terms', content: paymentTermsSection },
		{ content: body }
	]}
	{actions}
	loading={isPracticeOwnerOrAdmin && !error && status === undefined
		? 'Loading your Stripe Connect status'
		: undefined}
	loadError={error || undefined}
/>

<style>
	@layer components {
		ul {
			margin: 0;
			padding-inline-start: var(--space-6);
			color: var(--color-on-surface);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
		}

		li + li {
			margin-block-start: var(--space-2);
		}
	}
</style>
