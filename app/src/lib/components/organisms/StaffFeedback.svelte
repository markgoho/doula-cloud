<script lang="ts">
	/*
	 * The Pilot banner and its Feedback drawer, wired for the Staff shell
	 * (#1502, #1527): every signed-in Staff screen gets the same banner,
	 * drawer and send behavior from `PilotFeedback`, with the words that are
	 * the Staff side's own.
	 *
	 * `practiceName`/`roles` are optional and travel together: `/account`
	 * sits outside every Practice, and passes neither, which is exactly
	 * what no role line in the disclosure means.
	 */
	import type { ClientInput } from '#lib/feedback.js';
	import { rolesLabel } from '#lib/roles.js';
	import PilotFeedback from '#lib/components/organisms/PilotFeedback.svelte';

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
</script>

<PilotFeedback
	{email}
	bannerSentence=""
	controlText="Tell us what you think"
	drawerHeading="Send feedback to DoulaCloud"
	introText="The DoulaCloud team reads every piece of feedback during the pilot. It is how we decide what to fix first."
	destination="This goes to the DoulaCloud team, not to your Practice."
	sentBy="Mark Goho, who builds DoulaCloud,"
	{roleAndPractice}
	{onSend}
/>
