<script module lang="ts">
	export interface JourneyStep {
		label: string;
		href: string;
		status: 'completed' | 'current' | 'todo';
		/*
		 * The questions inside this step. Shown when the step is expanded,
		 * which is the whole reason the rail is a rail and not a progress
		 * bar: #432 wanted the journey legible at a glance on the summary
		 * page, and a step title alone does not say what was answered
		 * under it.
		 */
		questions?: { label: string; href: string }[];
	}
</script>

<script lang="ts">
	/*
	 * The journey rail that `QuestionPage` and `CheckAnswers` both carry
	 * (#464). Two identical consumers, which is exactly ADR-0018's bar for
	 * earning its own component; an organism rather than a molecule on
	 * #424's rule -- a molecule is a part of a section, an organism is a
	 * whole one, and this is a whole page region.
	 *
	 * ## It is a <nav>, and that is an amendment to ADR-0018
	 *
	 * ADR-0018 says a Template renders no navigation and no landmark. This
	 * renders both, on the account owner's call (#464). The rule's reason
	 * was that chrome is site-wide and session-derived, so a Template that
	 * rendered it could not be dropped into any route or rendered in a test
	 * with no session. A journey rail is neither: it is page-scoped, handed
	 * in as data by the route, and means nothing outside the one sequence
	 * it belongs to. The shell cannot render it, because the shell does not
	 * know the journey.
	 *
	 * #432's drawing asked for the <nav> *before* <main>. That is not
	 * available: #452 put <main> in the shell, so everything a Template
	 * renders is already inside it. A <nav> inside <main> is valid, is a
	 * landmark either way, and is what the account owner chose over a
	 * plain unnamed list.
	 *
	 * ## One presentation, disclosed by the reader (#585)
	 *
	 * This used to render two: a `.rail` of steps, and a `.strip` -- a
	 * summary line, a progress track and a "Show all steps" link -- with
	 * `@container (min-width: 67.5rem)` picking between them. That query
	 * is gone, and so is the second presentation.
	 *
	 * The query was trying to observe something CSS cannot see. Its hosts
	 * lay the journey out with `sidebar-l`, whose flex line wraps when the
	 * rail and the column stop fitting side by side, and **no selector
	 * reports that a wrap happened**. So the query was a second opinion
	 * about an event flexbox had already decided, and it drifted: measured
	 * on #585, `sidebar-l` pairs at 1078px while the query flipped at
	 * 1080px, and on `CheckAnswers` in its wide state it pairs at 955px --
	 * a 125px band where the layout opened a rail column and the component
	 * put a progress bar in it and left ~250px of the column empty. The
	 * 67.5rem literal was carried from a 60rem query both hosts had before
	 * #564 and lost to `sidebar-l`; it outlived its own mechanism, and
	 * nothing failed, because the coupling the check appeared to assert
	 * was never registered.
	 *
	 * The fix is not a better number. `<details>` moves the decision to
	 * the reader, which is the one party that can actually make it, and
	 * costs one line of vertical space when closed at every width. The
	 * strip's summary and track became the <summary>, so progress is
	 * legible without opening anything; the step list is what opens. See
	 * ADR-0024 rule 1 for the general rule this is the worked case of.
	 *
	 * ## Steps are data in
	 *
	 * The rail never computes its own steps. #432 requires that a Practice
	 * which has added no Client fields gets five steps rather than six with
	 * an empty one, and that derivation reads the Practice's own Client
	 * Field Template -- an endpoint that does not exist yet (#460). So the
	 * shape here is a typed array, and building it is the route's job
	 * (#466).
	 */
	import Icon from '#lib/components/atoms/Icon.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	interface Properties {
		/**
		 * Names the landmark, and captions the summary the reader opens.
		 */
		journey: string;
		steps: JourneyStep[];
		/**
		 * Which steps show their questions once the list is opened.
		 * `current` is a question page, where the reader is inside one step;
		 * `completed` is the summary page, where there is no current step and
		 * the whole answered journey is the point. A closed pair rather than
		 * a boolean, because these are two behaviors and not one thing
		 * switched off.
		 *
		 * There is no `open` prop: the list starts closed on every page
		 * (#2005). The summary page used to start it open, and with the rail
		 * in the column that put a list of five to nine steps, each saying
		 * "Completed", between Back and the title, above the answers that
		 * already show the whole journey.
		 */
		expand?: 'current' | 'completed';
	}

	let { journey, steps, expand = 'current' }: Properties = $props();

	const railId = $props.id();

	const statusLabels = {
		completed: 'Completed',
		current: 'In progress',
		todo: 'Not started'
	} as const;

	const currentStep = $derived(steps.find((step) => step.status === 'current'));
	const completedCount = $derived(steps.filter((step) => step.status === 'completed').length);

	/*
	 * Goal-Gradient and Parkinson, from the brief: a multi-step form has to
	 * show where the end is. On a question page that is the step number; on
	 * the summary page there is no current step, so it is the count.
	 */
	const summary = $derived(
		currentStep === undefined
			? `${completedCount} of ${steps.length} steps completed`
			: `Step ${steps.indexOf(currentStep) + 1} of ${steps.length} · ${currentStep.label}`
	);
	const completedPercent = $derived(
		steps.length === 0 ? 0 : Math.round((completedCount / steps.length) * 100)
	);
	const trackWidth = $derived(`${completedPercent}%`);

	function statusId(index: number) {
		return `${railId}-status-${index}`;
	}

	function isExpanded(step: JourneyStep) {
		return expand === 'completed' ? step.status === 'completed' : step.status === 'current';
	}
</script>

<!--
	One <details> inside the one <nav>. The <summary> is what the narrow
	strip used to be -- the step-or-count line and the progress track --
	so a reader who never opens it still knows where they are on every
	page. It starts closed on every page, so the indicator has one shape
	through the whole journey (#2005).

	No "Show all steps" link any more (#585). It existed to recover the
	step list the strip dropped, and nothing is dropped now. It was also
	passed by nothing outside the style guide, pointing at a route #466
	has not built.

	This stays a hand-written <details> rather than routing through
	`atoms/Details.svelte` (#1520). Its summary is not text -- it is the
	composed row above, a caption plus an `aria-hidden` progress track --
	and its marker and list are their own departure from the plain shape:
	the caret is drawn in the row rather than by the browser (#2005), and
	`details > ol` lays the steps out as a grid. Widening
	the atom to carry a composed summary and a bespoke marker position
	would fit one caller and no other; DataTable is the real caller the
	shared shape serves.
-->
<nav aria-label={journey}>
	<details>
		<summary>
			<!--
				The marker is an element of the row, not the browser's
				own (#2005, #1633). The native marker of an `outside`
				list item is drawn outside the <summary>'s border box,
				in the page gutter, and at 320px the page edge cut it in
				half; an `inside` one puts the block-level row on a line
				of its own below it. A caret in the row sits in the
				column, on the first line, beside the words, in every
				engine. It is `aria-hidden` (Icon draws no label): the
				<summary> already announces whether the list is open.
			-->
			<span class="summary-row">
				<span class="head">
					<span class="marker"><Icon name="caret-down" size={16} /></span>
					<span class="summary">{summary}</span>
				</span>
				<span class="track" aria-hidden="true">
					<span class="track-fill" style:inline-size={trackWidth}></span>
				</span>
			</span>
		</summary>

		<ol>
			<!--
				Keyed on index rather than on the href, the same call
				`CheckAnswers` and `DescriptionList` make on this ticket: a
				journey is an ordered sequence, a step has no identity beyond
				where it sits, and two entries that happen to share a
				destination are a duplicate key that throws rather than a
				collision that hides a row.
			-->
			{#each steps as step, index (index)}
				<li>
					<Link
						variant="step"
						href={step.href}
						label={step.label}
						current={step.status === 'current'}
						describedBy={statusId(index)}
					/>
					<p class="status" id={statusId(index)}>{statusLabels[step.status]}</p>
					{#if step.questions && isExpanded(step)}
						<ol class="questions">
							{#each step.questions as question, questionIndex (questionIndex)}
								<li>
									<Link variant="rail" href={question.href} label={question.label} />
								</li>
							{/each}
						</ol>
					{/if}
				</li>
			{/each}
		</ol>
	</details>
</nav>

<style>
	@layer components {
		ol {
			margin: 0;
			padding: 0;
			list-style: none;
		}

		/* No gap: each step's 2px leading bar meets its neighbor's, so the
		   column of transparent bars reads as one track down the list and
		   the active one lights a segment of it. That is the same reason
		   RecordDetail's contents list carries no gap. */
		details > ol {
			display: grid;
			margin-block-start: var(--space-3);
		}

		/*
		 * The browser's marker is off, in both the standard and the WebKit
		 * form, because the row draws its own (see the markup). The
		 * <summary> keeps no inline padding: the caret is the first thing
		 * in the row, so the rail starts at the column's edge.
		 */
		summary {
			list-style: none;
			padding-block: var(--space-2);
			cursor: pointer;
		}

		summary::-webkit-details-marker {
			display: none;
		}

		/* One line tall, with the caret centered in it, so it sits on the
		   caption's first line however many lines the caption takes. */
		.marker {
			display: inline-flex;
			align-items: center;
			block-size: 1lh;
			color: var(--color-on-surface-variant);
		}

		/*
		 * Closed, the caret points along the line (to the words); open, it
		 * points down to the list it opened -- the two states the native
		 * triangle showed. The icon turns and its box does not, so the turn
		 * never moves the caret out of its place in the row.
		 */
		.marker :global(svg) {
			transform: rotate(-90deg);
		}

		details[open] .marker :global(svg) {
			transform: none;
		}

		/*
		 * The caret and the words never part: a caption too long for its
		 * line wraps inside this pair rather than leaving the caret alone
		 * on a line above it.
		 */
		.head {
			display: flex;
			align-items: flex-start;
			font-size: var(--text-body-sm-size);
			gap: var(--space-2);
			min-inline-size: 0;
		}

		/* The arranging happens one level in, so the marker survives it. */
		summary .summary-row {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-3);
		}

		/*
		 * The track takes the room the caption leaves, and drops to its
		 * own line below `--page-rail` of it rather than squeezing to
		 * nothing -- a basis and a wrap, not a threshold: the caption is
		 * a step name and carries a Client's name, so how much it leaves
		 * is a question about the content and never about the window.
		 */
		summary .track {
			flex: 1 1 var(--page-rail);
		}

		/* Aligned under the step's own label rather than under its bar:
		   the status belongs to the words, and a status flush with the
		   track would read as another step. */
		.status {
			margin: 0 0 var(--space-3);
			padding-inline-start: calc(var(--space-4) + var(--border-active));
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		.questions {
			display: grid;
			margin-block-end: var(--space-3);
			padding-inline-start: var(--space-4);
		}

		/*
		 * A step label is a Practice's own free text -- "How to reach
		 * Anne-Marie Ochieng-Whitfield" -- so it gets this repo's answer
		 * for free text in a track that must not grow past its share:
		 * `min-inline-size: 0` to let it shrink below its content at all,
		 * and `overflow-wrap: anywhere` so a name with no spaces in it
		 * breaks rather than pushing the summary past its frame (#542,
		 * #725).
		 */
		.summary {
			min-inline-size: 0;
			overflow-wrap: anywhere;
			color: var(--color-on-surface);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-medium);
		}

		.track {
			display: block;
			block-size: var(--space-1);
			border-radius: var(--radius);
			background-color: var(--color-surface-container);
		}

		.track-fill {
			display: block;
			block-size: 100%;
			border-radius: var(--radius);
			background-color: var(--color-primary);
		}

		/*
		 * No query, and that is the point of #585. There is nothing left
		 * for one to decide: the step list is correct at every width --
		 * swept on /style-guide/step-rail, the nav never overflows its
		 * frame down to 144px, well under 320px -- so no content floor
		 * exists to derive, and the thing the old query was really trying
		 * to read was whether `sidebar-l` had wrapped, which no selector
		 * reports. ADR-0024 rule 1 now says so as a rule.
		 */
	}
</style>
