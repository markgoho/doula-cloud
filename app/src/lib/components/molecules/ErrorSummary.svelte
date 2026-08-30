<script module lang="ts">
	/**
	 * One refused thing, and where to fix it.
	 *
	 * `targetId` is optional, and that is a decision rather than a
	 * convenience. Most refusals belong to a control -- an address that is
	 * not an address, a name that is missing -- and those link to it. Some
	 * belong to the submission as a whole: the service was unreachable, or
	 * the server refused for a reason no field on the page caused. GOV.UK's
	 * own component renders such an entry as plain text rather than a link,
	 * because a link that goes nowhere useful is worse than no link. So the
	 * shape allows both, and a route that omits `targetId` is saying "no
	 * field on this page is what is wrong".
	 */
	export interface FormError {
		message: string;
		targetId?: string;
	}
</script>

<script lang="ts">
	/*
	 * GOV.UK's error summary (#467), the top half of the Recover from
	 * validation errors pattern: when a submit is refused, the page says so
	 * once, at the top, and lists every reason with a way to each one.
	 *
	 * ## Why a molecule
	 *
	 * A heading, a list and some links -- and, by #424's rule, *a part of a
	 * section rather than a whole one*: it is the failure region of one
	 * form, not the form. Owning focus behaviour does not move it up a
	 * tier; `MenuButton` is a molecule and owns the whole popover pairing,
	 * so behaviour is not the axis. Composition is, and this composes two
	 * atoms.
	 *
	 * ## Why it is not `Notice`
	 *
	 * `Notice` announces an outcome -- an invoice sent, a Contract saved,
	 * a delivery that failed. This announces that a *submission was
	 * refused* and lists the reasons. Keeping them apart is what stops a
	 * page growing two competing alert regions, and it is why the
	 * Engagement detail page keeps `Notice` (its failures are section-local
	 * operation outcomes, not a refused form) while every form in the app
	 * moves here.
	 *
	 * ## Focus, and the two halves of it
	 *
	 * On appear, this element takes focus, so the refusal is announced and
	 * the first fix is one Tab away. That is the half no markup can do, so
	 * it is the only script here.
	 *
	 * On activation, an entry moves focus to its control. That half is left
	 * to the platform, and it is not a gamble: HTML's own "scroll to the
	 * fragment" steps say that where the fragment's target is focusable,
	 * the browser runs the focusing steps on it. So `<a href="#id">`
	 * pointing at an input focuses that input, and there is no click
	 * handler to write -- the Rule of Least Power answer. govuk-frontend
	 * ships JavaScript for its own version because it does something more:
	 * it scrolls to a group's <legend> while focusing the first control
	 * inside it. We have no such case yet, and the day one arrives is the
	 * day to write the script, not before. Asserted against a real browser
	 * in `e2e/validation-recovery.e2e.ts` rather than left as a claim.
	 */
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	interface Properties {
		errors: FormError[];
	}

	let { errors }: Properties = $props();

	let summary = $state<HTMLDivElement>();

	/*
	 * Reading `errors` inside the effect is deliberate: a second refused
	 * submit that produces a different set has to announce itself again,
	 * and an effect that only ran on mount would leave the second failure
	 * silent for anyone who had tabbed away. A clean form never reaches
	 * here -- an empty array renders nothing at all -- so this cannot steal
	 * focus from someone who has not submitted.
	 */
	$effect(() => {
		void errors;
		summary?.focus();
	});
</script>

{#if errors.length > 0}
	<!--
		tabindex="-1" so the container can be focused programmatically but
		never lands in the tab order; the inner role="alert" is GOV.UK's own
		arrangement, which announces the list rather than the wrapper.
	-->
	<div class="summary" bind:this={summary} tabindex="-1">
		<div role="alert">
			<stack-l space="var(--space-3)">
				<!--
					The heading is fixed. It is not the server's text and not a
					variant: a reader who has seen it once knows what the box
					is before reading a word of it, which is the whole point of
					a named pattern (Jakob's Law, and the brief's own
					"conventional in pattern and behaviour").
				-->
				<Heading level={2} variant="section" text="There is a problem" />

				<!--
					Keyed on index, following #425/#464: two fields can be
					refused for the same reason, so a message is not an
					identity, and the list is positional anyway.
				-->
				<ul>
					<!-- v8 ignore start: only the compiled branch for "was this
					     keyed <li> added/removed from the DOM since the last
					     render" is unreachable here (Svelte's own each-block
					     diffing internals, not app code) -- both arms of the
					     loop body are exercised by "lists one entry per error,
					     each linking to its control" and "renders an entry with
					     no target as text rather than as a link" in
					     ErrorSummary.svelte.spec.ts -->
					{#each errors as error, index (index)}
						<li>
							{#if error.targetId}
								<Link href="#{error.targetId}" label={error.message} variant="error" />
							{:else}
								{error.message}
							{/if}
						</li>
					{/each}
					<!-- v8 ignore stop -->
				</ul>
			</stack-l>
		</div>
	</div>
{/if}

<style>
	@layer components {
		/*
		 * The error colour on the edge rather than the fill: the brief
		 * declares containers by an edge, and a solid red panel would
		 * outweigh the question it is about. `--border-active` rather than
		 * the `--border-thin` a `Notice` takes, because this one interrupts
		 * -- the emphatic step of the two the brief allows.
		 */
		.summary {
			padding: var(--space-5);
			border: var(--border-active) solid var(--color-error);
			border-radius: var(--radius);
			color: var(--color-on-surface);
		}

		/*
		 * The container is focused on appear and the ring has to be visible
		 * when it is -- :focus-visible would not fire, the focus being
		 * programmatic rather than from the keyboard.
		 */
		.summary:focus {
			outline: var(--focus-ring-width) solid var(--color-error);
			outline-offset: var(--focus-ring-offset);
		}

		ul {
			margin: 0;
			padding-inline-start: var(--space-5);
			font-size: var(--text-body-size);
		}

		li + li {
			margin-block-start: var(--space-2);
		}
	}
</style>
