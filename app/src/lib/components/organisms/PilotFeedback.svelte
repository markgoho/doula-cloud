<script lang="ts">
	/*
	 * The Pilot banner, its Feedback drawer and the Notice that follows a
	 * send (#1502), for the shell that hosts it: `StaffFeedback` (#1527) and
	 * `PortalFeedback` (#1528) each pass their own words and keep their own
	 * props, so the wiring lives here once.
	 *
	 * `onSend` owns the actual POST: the host layout closes its own
	 * `practiceId` or `engagementId` over it, so this organism never imports
	 * `#lib/api.js`.
	 *
	 * Each host layout wraps its feedback component in `{#key page.url.
	 * pathname}`: a navigation inside the layout never remounts it, so a
	 * draft, an open drawer or a just-sent Notice would otherwise follow a
	 * person from one screen to the next.
	 */
	import { feedbackSentNotice, type ClientInput } from '#lib/feedback.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Drawer from '#lib/components/organisms/Drawer.svelte';
	import FeedbackForm from '#lib/components/organisms/FeedbackForm.svelte';
	import PilotBanner from '#lib/components/molecules/PilotBanner.svelte';

	const idPrefix = $props.id();
	const drawerId = `${idPrefix}-drawer`;

	interface Properties {
		/**
		The sender's own email -- the Notice's "will email you at {email}".
		*/
		email: string;
		bannerSentence: string;
		controlText: string;
		drawerHeading: string;
		introText: string;
		destination: string;
		/**
		Who replies, in the Notice's sentence: "the DoulaCloud team".
		*/
		sentBy: string;
		/**
		The line in the disclosure naming the sender's role and Practice.
		*/
		roleAndPractice?: string;
		onSend: (input: ClientInput) => Promise<void>;
	}

	let {
		email,
		bannerSentence,
		controlText,
		drawerHeading,
		introText,
		destination,
		sentBy,
		roleAndPractice,
		onSend
	}: Properties = $props();

	let isOpen = $state(false);
	let sentMessage = $state<string | undefined>();
	let notice = $state<HTMLElement>();

	// The Notice takes focus once it appears (#1527, #1528), the same
	// programmatic-focus shape ErrorSummary's own effect uses.
	$effect(() => {
		void sentMessage;
		notice?.focus();
	});

	function handleSent(): void {
		isOpen = false;
		sentMessage = feedbackSentNotice(email, sentBy);
	}
</script>

<PilotBanner
	sentence={bannerSentence}
	{controlText}
	open={isOpen}
	controlsId={drawerId}
	onOpenFeedback={() => (isOpen = true)}
/>

{#if sentMessage}
	<div class="notice" bind:this={notice} tabindex="-1">
		<Notice variant="status" message={sentMessage} />
	</div>
{/if}

<Drawer id={drawerId} bind:open={isOpen} heading={drawerHeading}>
	<Text text={introText} />
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
		   :focus-visible. */
		.notice:focus {
			outline: var(--focus-ring-width) solid var(--color-status);
			outline-offset: var(--focus-ring-offset);
		}
	}
</style>
