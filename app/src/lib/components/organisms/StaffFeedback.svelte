<script lang="ts">
	/*
	 * The Pilot banner and its Feedback drawer, wired for the Staff shell
	 * (#1502, #1527): every signed-in Staff screen gets the same banner,
	 * drawer and send behavior, so this is the one place that wiring
	 * lives rather than a copy inside `practices/+layout.svelte` and a
	 * second inside `account/+layout.svelte`.
	 *
	 * `onSend` owns the actual POST, the same way `FeedbackForm`'s own
	 * `onSend` does one level down (`RefundPaymentForm.onConfirm`'s
	 * contract, #1527): each layout calls `sendStaffFeedback` itself with
	 * its own `practiceId` closed over, so this organism never imports
	 * `#lib/api.js` and the `/style-guide/staff-feedback` demo below can
	 * hand it a harmless stub instead of ever reaching a real endpoint.
	 *
	 * `practiceName`/`roles` are optional and travel together: `/account`
	 * sits outside every Practice, and passes neither, which is exactly
	 * what no role line in the disclosure means.
	 *
	 * Both host layouts persist across a navigation inside their own tree
	 * (Clients to Schedule never remounts `practices/+layout.svelte`), so
	 * a Feedback draft, an open drawer or a just-sent Notice would
	 * otherwise silently follow a person from one screen to the next.
	 * Each layout wraps this component itself in `{#key page.url.
	 * pathname}` rather than this component resetting its own state by
	 * hand: a navigation is "leaving the screen" (#1527's own AC for the
	 * draft), and remounting is what that AC asks for regardless of
	 * whether the drawer was open, mid-draft, or just showed a Notice --
	 * one seam covers all three rather than a state variable per one.
	 */
	import { feedbackSentNotice, type ClientInput } from '#lib/feedback.js';
	import { rolesLabel } from '#lib/roles.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Drawer from '#lib/components/organisms/Drawer.svelte';
	import FeedbackForm from '#lib/components/organisms/FeedbackForm.svelte';
	import PilotBanner from '#lib/components/molecules/PilotBanner.svelte';

	// Per-instance, the same shape RadioGroup/Textarea/LabeledField already
	// generate their own ids with -- two instances on one screen (the
	// style-guide page's own "under a Practice"/"under /account" pair)
	// must not collide on the same dialog id.
	const idPrefix = $props.id();
	const drawerId = `${idPrefix}-drawer`;

	interface Properties {
		/** The sender's own email -- the Notice's "will email you at
		 * {email}" (#1502 Q4's table). */
		email: string;
		practiceName?: string;
		roles?: string[];
		/** Posts the send -- the caller's own `sendStaffFeedback` call,
		 * with its `practiceId` already closed over. */
		onSend: (input: ClientInput) => Promise<void>;
	}

	let { email, practiceName, roles, onSend }: Properties = $props();

	const roleAndPractice = $derived(
		practiceName !== undefined && roles !== undefined && roles.length > 0
			? `${rolesLabel(roles)}, ${practiceName}`
			: undefined
	);

	let isOpen = $state(false);
	let sentMessage = $state<string | undefined>();
	let notice = $state<HTMLElement>();

	// The Notice takes focus once it appears (#1527's own AC), the same
	// shape ErrorSummary's own effect uses for the same reason -- a
	// programmatic focus, not a keyboard one, so nothing here waits on
	// :focus-visible.
	$effect(() => {
		void sentMessage;
		notice?.focus();
	});

	function handleSent(): void {
		isOpen = false;
		// feedbackSentNotice (#1528) is shared with PortalFeedback.svelte's
		// own identical Notice, and is what makes "no address known" -- an
		// async, best-effort session read can fail on either shell -- a
		// graceful "Feedback sent. Thank you." rather than the broken
		// sentence a bare template would interpolate nothing into.
		sentMessage = feedbackSentNotice(email, 'Mark Goho, who builds Doula Cloud,');
	}
</script>

<PilotBanner
	sentence="Doula Cloud is new, and you are one of the first to use it."
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

<Drawer id={drawerId} bind:open={isOpen} heading="Send feedback to Doula Cloud">
	<Text
		text="The Doula Cloud team reads every piece of feedback during the pilot. It is how we decide what to fix first."
	/>
	<FeedbackForm
		legend="What kind of feedback is it?"
		errorKind="Select what kind of feedback it is"
		textLabel="Tell us more"
		textHint="What were you trying to do, and what happened?"
		destination="This goes to the Doula Cloud team, not to your Practice."
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
		   :focus-visible -- the same reason ErrorSummary's own ring is
		   unconditional. */
		.notice:focus {
			outline: var(--focus-ring-width) solid var(--color-status);
			outline-offset: var(--focus-ring-offset);
		}
	}
</style>
