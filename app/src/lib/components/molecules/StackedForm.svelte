<script lang="ts">
	/*
	 * A form that stacks its own fields (#660).
	 *
	 * `stack-l`'s rule is `> * + *`: it spaces a child and never a
	 * grandchild. A Template stacks the top-level siblings of the region it
	 * hands a route, and `LabeledField` stacks its own label, hint and
	 * control -- but nothing between those two levels stacks a form's
	 * fields, so a `<form>` that holds `LabeledField`s directly rendered
	 * them flush: the Password label against the Email input, the submit
	 * button against the last field. Every unauthenticated entry screen in
	 * the app had that shape, because ADR-0018 leaves region-internal
	 * arrangement to the page and every one of those pages arranged it the
	 * same way.
	 *
	 * `var(--space-5)` is the token `FormPage` already spends on a
	 * fieldset's content, so a form reads the same whichever archetype it
	 * is on.
	 *
	 * `novalidate` is not a prop. GOV.UK's Recover from validation errors
	 * pattern (ADR-0021) is that the page refuses the submit and says so
	 * once, at the top: the browser's own bubbles refuse before the page
	 * can, vanish on the next keystroke, and are worded by the browser
	 * rather than by us. `required` stays on the controls, because it is a
	 * true statement about the field and assistive technology reads it;
	 * what it no longer does is block. A form that wants the browser's
	 * refusal instead is not this component.
	 */
	import type { Snippet } from 'svelte';

	interface Properties {
		/**
		 * The submit handler. It owns `preventDefault()` -- this component
		 * does not call it, because a form that means to let the browser
		 * navigate is a decision for the page, not for its spacing.
		 */
		onSubmit: (event: SubmitEvent) => void;
		/**
		 * The fields, and whatever else the form asks for, in source order.
		 */
		children: Snippet;
	}

	let { onSubmit, children }: Properties = $props();
</script>

<form onsubmit={onSubmit} novalidate>
	<stack-l space="var(--space-5)">
		{@render children()}
	</stack-l>
</form>
