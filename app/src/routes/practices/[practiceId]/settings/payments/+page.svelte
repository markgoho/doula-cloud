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
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadConnectStatus, connect, type ConnectStatus, type ConnectStatusResult } from '#lib/payments.js';
	import { loadWebsite, type PracticeWebsite } from '#lib/website.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';

	let status = $state<ConnectStatusResult | undefined>();
	let website = $state<PracticeWebsite | undefined>();
	let error = $state('');
	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));
	let connectParameter = $derived(page.url.searchParams.get('connect'));

	let connectError = $state('');
	let isConnecting = $state(false);

	onMount(async () => {
		try {
			status = await loadConnectStatus(apiFetchWithSession, page.params.practiceId!);
			// Every Staff member may read this (#440), so it loads for
			// everyone: a Doula who opens this screen should see why Clients
			// cannot pay yet rather than an unexplained missing button.
			website = await loadWebsite(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Stripe Connect status';
			return;
		}

		// The connect button's enabled state mirrors the "owner"-role gating
		// the billing page already uses -- server-side enforcement
		// (RequireOwner) is what actually matters, this is UX only.
		const sessionResponse = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/session`);
		if (sessionResponse.ok) {
			const body: { roles: string[] } = await sessionResponse.json();
			roles = body.roles;
		}
	});

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
			explanation: 'Connect Stripe so Clients can pay their invoices.',
			onboarding: 'always'
		},
		onboarding_incomplete: {
			label: 'Onboarding incomplete',
			variant: 'warning',
			explanation: 'Stripe still needs some details before Clients can pay you.',
			onboarding: 'always'
		},
		pending: {
			label: 'Awaiting Stripe review',
			variant: 'warning',
			explanation: 'Stripe is reviewing the details you submitted. Nothing is needed from you.',
			onboarding: 'never'
		},
		payouts_restricted: {
			label: 'Taking payments, payouts on hold',
			variant: 'warning',
			explanation:
				'Clients can pay their invoices, but Stripe cannot send the money to your bank yet.',
			onboarding: 'if-outstanding'
		},
		active: {
			label: 'Active',
			variant: 'success',
			explanation: 'Clients can pay their invoices and payouts reach your bank.',
			onboarding: 'never'
		}
	};

	let copy = $derived(status === undefined ? undefined : statusCopy[status.status]);

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

{#snippet body()}
	<!--
		A non-null assertion, not another `{#if status}`: `body` is one of
		`FormPage`'s `fieldsets`, which only render once neither `loadError`
		nor `loading` is active below, and `loading` is exactly `status ===
		undefined` -- so `status` is always defined by the time this runs.
		A second guard here would compile to a branch that never takes its
		false path, which the coverage gate would then refuse.
	-->
	<cluster-l>
		<Text text="Stripe Connect status:" />
		<Badge label={statusCopy[status!.status].label} variant={statusCopy[status!.status].variant} />
	</cluster-l>

	{#if isBackFromStripeRestricted}
		<Notice
			variant="error"
			message="Stripe still needs something from you before Clients can pay. Open the form again below and finish what it asks for."
		/>
	{:else if connectParameter === 'return'}
		<Notice
			variant="status"
			message="Stripe onboarding finished. Status updates once Stripe confirms your account is active."
		/>
	{:else if connectParameter === 'refresh'}
		<Notice variant="status" message="Your Stripe onboarding link expired. Start again below." />
	{/if}

	<Text text={statusCopy[status!.status].explanation} />

	<!-- The count, not the list. requirementsDue holds Stripe's own
	machine-readable field paths ("configuration.merchant.mcc"), which
	name nothing an Owner recognizes. The place those get asked in words
	is Stripe's hosted form, which the button below opens; the paths stay
	in the database for the audit trail. -->
	{#if status!.requirementsDue.length > 0}
		<Text
			text={status!.requirementsDue.length === 1
				? 'Stripe needs 1 more detail from you.'
				: `Stripe needs ${status!.requirementsDue.length} more details from you.`}
		/>
	{/if}

	{#if canStartOnboarding && !hasDeclaredWebsite}
		<!--
			Block, do not warn. A disabled button with a tooltip would leave
			her guessing what unlocks it; this names the missing thing and
			links to where she supplies it. PostConnectHandler refuses the
			request as well, which is what actually holds the line.
		-->
		<Notice
			variant="info"
			message="Stripe will not let you take Client payments until it can see where you are online. Tell us your website or let us publish a page for you, then come back here."
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
			message="The page we publish for you is not loading, so Stripe would find nothing at your web address. Open your website settings and publish it again."
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
{/snippet}

{#snippet actions()}
	{#if canConnect}
		{#if isOwner}
			<Button
				label={status!.status === 'not_connected' ? 'Connect Stripe' : 'Continue Stripe onboarding'}
				onClick={handleConnect}
				loading={isConnecting}
			/>
			{#if connectError}
				<Notice variant="error" message={connectError} />
			{/if}
		{:else}
			<Text text="Ask a Practice Owner to connect Stripe." />
		{/if}
	{/if}
{/snippet}

<FormPage
	title="Payments"
	fieldsets={[{ content: body }]}
	{actions}
	loading={!error && status === undefined ? 'Loading your Stripe Connect status' : undefined}
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
