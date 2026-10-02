<script lang="ts">
	/*
	 * The Pilot banner and its Feedback drawer, wired for the Portal shell
	 * (#1502, #1528): every signed-in Portal screen gets the same banner,
	 * drawer and send behavior -- the Portal's own instance of the wiring
	 * `organisms/StaffFeedback.svelte` (#1527) already built for the
	 * Staff shell, reusing the shell-neutral `FeedbackForm` the same way.
	 *
	 * `onSend` owns the actual POST, the same contract StaffFeedback's own
	 * `onSend` carries: the host layout calls `sendPortalFeedback` itself
	 * with its own `engagementId` already closed over, so this organism
	 * never imports `#lib/api.js`.
	 *
	 * `practiceName` is optional (#1528's own AC): a Portal Account
	 * reaches Clients at more than one Practice (ADR-0015), so a screen
	 * with no single Engagement in view -- today, only the brief window
	 * before `engagements/[engagementId]/+layout.ts`'s load resolves --
	 * has no one Practice to name, and the copy below reads "your doula's
	 * Practice" in its place rather than waiting on a value that may
	 * never narrow to one.
	 *
	 * The host layout wraps this component itself in `{#key page.url.
	 * pathname}`, the same reason `StaffFeedback`'s own doc comment gives:
	 * a navigation inside `engagements/[engagementId]` never remounts
	 * `portal/(authenticated)/+layout.svelte`, so a draft, an open drawer
	 * or a just-sent Notice would otherwise follow a person from one
	 * screen to the next.
	 */
	import { feedbackSentNotice, type ClientInput } from '#lib/feedback.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Drawer from '#lib/components/organisms/Drawer.svelte';
	import FeedbackForm from '#lib/components/organisms/FeedbackForm.svelte';
	import PilotBanner from '#lib/components/molecules/PilotBanner.svelte';

	// Per-instance, the same reason StaffFeedback's own prefix exists --
	// there is no second instance on one Portal screen today, but a dialog
	// id must not collide if there ever is.
	const idPrefix = $props.id();
	const drawerId = `${idPrefix}-drawer`;

	interface Properties {
		/** The Portal Account's own sign-in address -- the Notice's "will
		 * email you at {email}" (#1502 Q4's table). */
		email: string;
		/** The Practice this screen sits inside, when it sits inside one
		 * (#1528's own AC: absent on a screen with no single Practice). */
		practiceName?: string;
		/** Posts the send -- the host layout's own `sendPortalFeedback`
		 * call, with its `engagementId` already closed over. */
		onSend: (input: ClientInput) => Promise<void>;
	}

	let { email, practiceName, onSend }: Properties = $props();

	// "Client, {Practice name}" mirrors StaffFeedback's own "{roles},
	// {practiceName}" line; omitted entirely with no Practice to name,
	// the same as StaffFeedback does under /account.
	const roleAndPractice = $derived(practiceName === undefined ? undefined : `Client, ${practiceName}`); // voice:ignore: the team reads this label, not the Client

	const intro = $derived(
		practiceName === undefined
			? "Your doula's Practice uses Doula Cloud to run this portal. The Doula Cloud team reads every piece of feedback."
			: `${practiceName} uses Doula Cloud to run this portal. The Doula Cloud team reads every piece of feedback.`
	);

	const destination = $derived(
		practiceName === undefined
			? "This goes to the Doula Cloud team, not to your doula's Practice. For anything about your care, message your doula."
			: `This goes to the Doula Cloud team, not to ${practiceName}. For anything about your care, message your doula.`
	);

	let isOpen = $state(false);
	let sentMessage = $state<string | undefined>();
	let notice = $state<HTMLElement>();

	// The Notice takes focus once it appears (#1528's own AC, matching
	// #1527's), the same programmatic-focus shape ErrorSummary's own
	// effect uses.
	$effect(() => {
		void sentMessage;
		notice?.focus();
	});

	function handleSent(): void {
		isOpen = false;
		sentMessage = feedbackSentNotice(email, 'the Doula Cloud team');
	}
</script>

<PilotBanner
	sentence="This care portal is new."
	controlText="Tell us what is not working or what you need"
	open={isOpen}
	controlsId={drawerId}
	onOpenFeedback={() => (isOpen = true)}
/>

{#if sentMessage}
	<div class="notice" bind:this={notice} tabindex="-1">
		<Notice variant="status" message={sentMessage} />
	</div>
{/if}

<Drawer id={drawerId} bind:open={isOpen} heading="Send feedback about this portal">
	<Text text={intro} />
	<FeedbackForm
		legend="What kind of feedback is it?"
		errorKind="Select what kind of feedback it is"
		textLabel="Tell us more"
		textHint="What were you trying to do, and what happened?"
		{destination}
		detailsSummary="What else we send with your feedback"
		{roleAndPractice}
		{onSend}
		onSent={handleSent}
	/>
</Drawer>

<style>
	@layer components {
		.notice {
			margin: var(--space-4) var(--page-gutter) 0;
		}

		/* Focused programmatically on appear (the effect above), not from
		   the keyboard, so this needs its own ring rather than relying on
		   :focus-visible -- the same reason StaffFeedback's own ring is
		   unconditional. */
		.notice:focus {
			outline: var(--focus-ring-width) solid var(--color-status);
			outline-offset: var(--focus-ring-offset);
		}
	}
</style>
