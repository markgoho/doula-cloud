<script lang="ts">
	import StepRail, { type JourneyStep } from '#lib/components/organisms/StepRail.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * step is named after the question it asks, and those questions carry
	 * the Client's name -- so the rail's width is decided by the longest
	 * Client name a Practice has.
	 */
	const steps: JourneyStep[] = [
		{
			label: 'Who Anne-Marie Ochieng-Whitfield is',
			href: '/style-guide/step-rail',
			status: 'completed',
			questions: [
				{ label: "Anne-Marie Ochieng-Whitfield's full legal name", href: '/style-guide/step-rail' },
				{ label: 'Date of birth', href: '/style-guide/step-rail' }
			]
		},
		{
			label: 'How to reach Anne-Marie Ochieng-Whitfield',
			href: '/style-guide/step-rail#reach',
			status: 'current',
			questions: [
				{ label: 'Email address we send the portal invite to', href: '/style-guide/step-rail' },
				{ label: 'Phone number', href: '/style-guide/step-rail' }
			]
		},
		{
			label: 'Where Anne-Marie Ochieng-Whitfield lives',
			href: '/style-guide/step-rail#lives',
			status: 'todo'
		},
		{ label: 'Birth preferences', href: '/style-guide/step-rail#prefs', status: 'todo' },
		{ label: 'Check your answers', href: '/style-guide/step-rail#check', status: 'todo' }
	];

	const completed: JourneyStep[] = steps.map((step) => ({ ...step, status: 'completed' }));
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Step rail" />
	<Text
		text="The journey QuestionPage and CheckAnswers both carry. It is a named nav landmark holding one disclosure: the summary line and progress track are always visible, and the step list is what opens. No width decides anything (#585) — the summary page opens it because the journey is the point there, a question page leaves it closed because the question is, and after that it is the reader's."
		tone="variant"
	/>

	<!--
		Three demos, one per render branch, which is #720's shape rule: the
		closed state, the open state, and the open state with no room. A
		fixture that showed only the closed one would sweep a <details>
		whose entire contents are out of layout.
	-->
	<Heading level={3} variant="card" text="On a question page (expand=current, closed)" />
	<container-l>
		<div class="frame">
			<StepRail journey="Adding a Client to Highland Midwifery" {steps} />
		</div>
	</container-l>

	<Heading level={3} variant="card" text="On the summary page (expand=completed, open)" />
	<container-l>
		<div class="frame">
			<StepRail
				journey="Adding a Client to Highland Midwifery"
				steps={completed}
				expand="completed"
			/>
		</div>
	</container-l>

	<Heading level={3} variant="card" text="Open, with no column to sit in" />
	<div class="narrow">
		<container-l>
			<StepRail
				journey="Adding a Client to Highland Midwifery"
				steps={completed}
				expand="completed"
			/>
		</container-l>
	</div>
</stack-l>

<style>
	@layer components {
		/* The journey sits in a --page-rail column on CheckAnswers, so it
		   needs that width to sit in before it looks like anything. */
		.frame {
			max-inline-size: var(--page-rail);
		}

		/* Wider than the rail and narrower than a paired layout: where the
		   journey lands once sidebar-l has wrapped it above the content. */
		.narrow {
			max-inline-size: var(--form-max);
		}
	}
</style>
