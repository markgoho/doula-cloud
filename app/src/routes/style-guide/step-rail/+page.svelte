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
		text="The journey rail QuestionPage and CheckAnswers both carry. It is a named nav landmark; below the Templates' 60rem threshold it becomes a one-line strip with a track, because there is no column beside the content to put it in."
		tone="variant"
	/>

	<Heading level={3} variant="card" text="On a question page (expand=current)" />
	<container-l>
		<div class="frame">
			<StepRail journey="Adding a Client to Highland Midwifery" {steps} allStepsHref="/style-guide/step-rail" />
		</div>
	</container-l>

	<Heading level={3} variant="card" text="On the summary page (expand=completed)" />
	<container-l>
		<div class="frame">
			<StepRail
				journey="Adding a Client to Highland Midwifery"
				steps={completed}
				expand="completed"
				allStepsHref="/style-guide/step-rail"
			/>
		</div>
	</container-l>

	<Heading level={3} variant="card" text="Narrow, where the rail has no column" />
	<div class="narrow">
		<container-l>
			<StepRail
				journey="Adding a Client to Highland Midwifery"
				{steps}
				allStepsHref="/style-guide/step-rail"
			/>
		</container-l>
	</div>
</stack-l>

<style>
	@layer components {
		/* The rail is a grid column in a Template, so it needs a width to
		   sit in before it looks like anything. */
		.frame {
			max-inline-size: var(--page-rail);
		}

		/* Under the 60rem container threshold, so the strip shows instead. */
		.narrow {
			max-inline-size: var(--form-max);
		}
	}
</style>
