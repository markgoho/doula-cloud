<script lang="ts">
	import Select from '#lib/components/atoms/Select.svelte';
	import { WORK_STATE_NAMES } from '#lib/workStates.js';

	interface Properties {
		/**
		 * The full state name, e.g. "New York". Bind, then convert with
		 * workStateCode() before sending it to the API, which stores the
		 * USPS two-letter code.
		 */
		value: string;
	}

	let { value = $bindable('') }: Properties = $props();

	const uid = $props.id();
	const hintId = `${uid}-hint`;
</script>

<!--
	Asked at onboarding on every path a person joins by (#415). The reason
	is stated because we are asking a personal question for a reason that
	is not obvious, and the last sentence closes off the question it
	invites -- no, we do not want your address.

	A native <select> over 51 fixed options: the Rule of Least Power answer,
	and it needs no JavaScript to be usable. The hint is wired to the
	control with aria-describedby rather than merely sitting near it, so a
	screen reader reads the reason along with the question.
-->
<stack-l>
	<label for={uid}>Which state do you work from?</label>
	<p id={hintId} class="hint">
		Sales tax on your practice's credits is worked out from where its team works &mdash; so this
		needs to be right, and needs updating if you move. We only need the state, not your address.
	</p>
	<Select
		id={uid}
		describedBy={hintId}
		options={[...WORK_STATE_NAMES]}
		placeholder="Choose a state"
		bind:value
		required
	/>
</stack-l>

<style>
	@layer components {
		.hint {
			margin: 0;
			color: var(--color-muted);
			font-size: var(--text-sm);
		}
	}
</style>
