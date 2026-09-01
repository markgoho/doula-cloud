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
		text="The journey rail QuestionPage and CheckAnswers both carry. It is a named nav landmark: a progress summary and track, then the full step list, both unconditional (#564) -- there is no narrower alternative to switch to, so it reads the same whether it sits in a --page-rail column beside the form or stacks above it at any other width."
		tone="variant"
	/>

	<Heading level={3} variant="card" text="On a question page (expand=current)" />
	<container-l>
		<div class="frame">
			<StepRail journey="Adding a Client to Highland Midwifery" {steps} />
		</div>
	</container-l>

	<Heading level={3} variant="card" text="On the summary page (expand=completed)" />
	<container-l>
		<div class="frame">
			<StepRail journey="Adding a Client to Highland Midwifery" steps={completed} expand="completed" />
		</div>
	</container-l>

	<Heading level={3} variant="card" text="At the width a stacked page gives it" />
	<div class="narrow">
		<container-l>
			<StepRail journey="Adding a Client to Highland Midwifery" {steps} />
		</container-l>
	</div>
</stack-l>

<style>
	@layer components {
		/* The rail sits in a --page-rail column beside a Template's own
		   form, so it needs a width to sit in before it looks like
		   anything. */
		.frame {
			max-inline-size: var(--page-rail);
		}

		/* The width a stacked page's own column gives it instead, once the
		   Template's sidebar-l cannot keep the form at --form-max. */
		.narrow {
			max-inline-size: var(--form-max);
		}
	}
</style>
