<!--
@component
The teaser's card: the waitlist form beside the letter (#358). Copy word
for word from #362.

The form is plain HTML that posts to Buttondown, and the visitor goes with
it: nothing intercepts the submit and nothing calls `fetch`, because
Buttondown sometimes answers with a CAPTCHA or a validation error that the
visitor has to see (#362). `type="email"` and `required` are the page's
only refusal; there is no server of ours behind it (ADR-0014).

With no `action` there is no form, because a form with nowhere to post
would take a person's name and lose it. The card then says the list is
not open yet. #366 records the address; `WAITLIST_FORM_ACTION` carries it.
-->
<script lang="ts">
	import { PRODUCT_NAME } from '#lib/product.js';
	import { UTM_PARAMETERS, utmScriptElement } from '#lib/waitlist.js';

	interface Properties {
		// Buttondown's embed URL, or nothing while #366 is open.
		action?: string;
	}

	let { action }: Properties = $props();

	// Several expressions below are written as one template string, and the
	// same in the attribute. Svelte compiles a bare expression with a `??
	// ''` fallback that no value here can reach, which the coverage gate
	// reads as an untested branch; a template string is already a string.
</script>

<section class="card" aria-labelledby="waitlist-heading"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
	<stack-l>
		<h2 id="waitlist-heading">Join the waitlist</h2>
		<p class="note">Two fields. One email from me when it opens.</p>

		{#if action}
			<form method="post" {action}>
				<stack-l>
					<div class="field">
						<label for="waitlist-first-name">First name</label>
						<input
							id="waitlist-first-name"
							name="metadata__first_name"
							type="text"
							autocomplete="given-name"
						/>
					</div>
					<div class="field">
						<label for="waitlist-email">Email address</label>
						<input
							id="waitlist-email"
							name="email"
							type="email"
							autocomplete="email"
							inputmode="email"
							required
						/>
					</div>
					{#each UTM_PARAMETERS as parameter (parameter)}
						<input type="hidden" name={`metadata__${parameter}`} value="" />
					{/each}
					<button type="submit">Add me to the list</button>
				</stack-l>
			</form>
			<!-- ADR-0016's first-party script: it copies the address bar's
			     utm_* values into the hidden inputs above. Inline, after the
			     inputs it fills, because the site ships no bundle. -->
			<!-- eslint-disable-next-line svelte/no-at-html-tags -- a fixed string of our own, not visitor input -->
			{@html utmScriptElement}
			<p class="fine">
				One confirmation email now, one when we open. Nothing else, and you can leave the list from any
				of them. <a href="/privacy">Privacy Policy</a>
			</p>
		{:else}
			<p class="closed">
				{`The list isn't open yet. It opens here in the next few days, and ${PRODUCT_NAME} opens in January 2027.`}
			</p>
		{/if}
	</stack-l>
</section>

<style>
	.card {
		padding: var(--space-6);
		border: var(--border-thin) solid var(--color-outline-variant);
		border-radius: var(--radius);
		background-color: var(--color-surface-bright);
	}

	h2 {
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
		line-height: var(--text-heading-leading);
		letter-spacing: var(--text-heading-tracking);
	}

	.note,
	.fine,
	.closed {
		color: var(--color-on-surface-variant);
	}

	.fine {
		font-size: var(--text-meta-size);
		line-height: var(--text-meta-leading);
		letter-spacing: var(--text-meta-tracking);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	label {
		font-weight: var(--text-label-weight);
	}

	input {
		padding: var(--space-3);
		border: var(--border-thin) solid var(--color-outline);
		border-radius: var(--radius);
		background-color: var(--color-surface);
		color: var(--color-on-surface);
		font: inherit;
	}

	button {
		padding: var(--space-3) var(--space-5);
		border: 0;
		border-radius: var(--radius);
		background-color: var(--color-primary);
		color: var(--color-on-primary);
		font: inherit;
		font-weight: var(--text-label-weight);
		cursor: pointer;
	}

	button:hover {
		background-color: var(--color-primary-hover);
	}

	input:focus-visible,
	button:focus-visible,
	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}

	a {
		color: var(--color-primary);
	}
</style>
