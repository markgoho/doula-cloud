<script lang="ts">
	/*
	 * The Pilot banner and its Feedback drawer, wired for the Portal shell
	 * (#1502, #1528): every signed-in Portal screen gets the same banner,
	 * drawer and send behavior from `PilotFeedback`, with the words that are
	 * the Portal's own.
	 *
	 * `practiceName` is optional (#1528's own AC): a Portal Account
	 * reaches Clients at more than one Practice (ADR-0015), so a screen
	 * with no single Engagement in view -- today, only the brief window
	 * before `engagements/[engagementId]/+layout.ts`'s load resolves --
	 * has no one Practice to name, and the copy below reads "your doula's
	 * Practice" in its place rather than waiting on a value that may
	 * never narrow to one.
	 */
	import type { ClientInput } from '#lib/feedback.js';
	import PilotFeedback from '#lib/components/organisms/PilotFeedback.svelte';

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

	const roleAndPractice = $derived(practiceName === undefined ? undefined : `Client, ${practiceName}`); // voice:ignore: the team reads this label, not the Client

	const intro = $derived(
		practiceName === undefined
			? "Your doula's Practice uses DoulaCloud to run this portal. The DoulaCloud team reads every piece of feedback."
			: `${practiceName} uses DoulaCloud to run this portal. The DoulaCloud team reads every piece of feedback.`
	);

	const destination = $derived(
		practiceName === undefined
			? "This goes to the DoulaCloud team, not to your doula's Practice. For anything about your care, message your doula."
			: `This goes to the DoulaCloud team, not to ${practiceName}. For anything about your care, message your doula.`
	);
</script>

<PilotFeedback
	{email}
	bannerSentence="This care portal is new."
	controlText="Tell us what is not working or what you need"
	drawerHeading="Send feedback about this portal"
	introText={intro}
	{destination}
	sentBy="the DoulaCloud team"
	{roleAndPractice}
	{onSend}
/>
