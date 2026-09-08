<script lang="ts">
	import BirthOutcomeSection from '#lib/components/organisms/BirthOutcomeSection.svelte';
	import type { BirthOutcomeResult } from '#lib/engagementDetail.js';

	/*
	 * Five states, because every one of them is a different screen and a
	 * style-guide entry that shows one hides the rest (govuk-alignment.md,
	 * rule 4): nothing recorded, a recorded live birth an Owner may
	 * correct, a recorded loss read by somebody who may not, and the same
	 * unrecorded state as a contractor Doula sees it -- no control at all.
	 * The question itself opens behind a control, so the sweep mounts
	 * these five with the form closed; the form's own two controls are
	 * swept on `/style-guide/radio-group` and `/style-guide/date-fields`,
	 * and the composition was walked by hand at 320px (govuk-alignment.md).
	 *
	 * The recorded pair is a `loss` in one of them on purpose. That is the
	 * hardest thing this section ever says, and the sweep should measure
	 * the sentence a Practice will actually read rather than the cheerful
	 * one (ADR-0025).
	 */
	const recorded: BirthOutcomeResult = {
		kind: 'recorded',
		facts: { birthOutcome: 'live_birth', pregnancyEndedOn: '2026-08-14' }
	};
	const frozen: BirthOutcomeResult = {
		kind: 'confirmable',
		message:
			'this Engagement already has a birth outcome; only a Practice Owner can correct it'
	};
</script>

<stack-l space="var(--space-6)">
	<h1>Birth outcome section</h1>

	<section>
		<h2>Nothing recorded, and this reader may record it</h2>
		<BirthOutcomeSection canRecord canCorrect={false} onRecord={async () => recorded} />
	</section>

	<section>
		<h2>Nothing recorded, and the row is already frozen elsewhere</h2>
		<p>Submitting here answers with the press-through an Owner confirms.</p>
		<BirthOutcomeSection canRecord canCorrect onRecord={async () => frozen} />
	</section>

	<section>
		<h2>Recorded, read by a Practice Owner</h2>
		<BirthOutcomeSection
			outcome="live_birth"
			endedOn="2026-08-14"
			canRecord
			canCorrect
			onRecord={async () => recorded}
		/>
	</section>

	<section>
		<h2>Recorded, read by somebody who may not correct it</h2>
		<BirthOutcomeSection
			outcome="loss"
			endedOn="2026-08-14"
			canRecord
			canCorrect={false}
			onRecord={async () => recorded}
		/>
	</section>

	<section>
		<h2>A contractor Doula, who is offered nothing</h2>
		<BirthOutcomeSection canRecord={false} canCorrect={false} onRecord={async () => recorded} />
	</section>
</stack-l>
