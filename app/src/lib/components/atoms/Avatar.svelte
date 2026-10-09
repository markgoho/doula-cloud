<script module lang="ts">
	/*
	 * Initials are taken from the two name fields, never guessed from white
	 * space (#1537): "Mary Anne Smith" is first name "Mary Anne" and last
	 * name "Smith", and a split on spaces cannot know that. They are derived
	 * here rather than served, because both names are already on the wire
	 * (`GET /api/staff/session`) and a second field holding two letters of
	 * them would be a copy that can go stale.
	 *
	 * One letter from each. A caller that has only one name (the Client
	 * portal knows the Client by the name she goes by) leaves `lastName`
	 * empty and gets one letter. Exported so the style guide and the specs
	 * use the same rule as the component.
	 */
	export function initialsOf(firstName: string, lastName = ''): string {
		const first = firstName.trim().slice(0, 1);
		const last = lastName.trim().slice(0, 1);
		return (first + last).toLocaleUpperCase();
	}
</script>

<script lang="ts">
	interface Properties {
		firstName: string;
		lastName?: string;
	}

	let { firstName, lastName = '' }: Properties = $props();

	const initials = $derived(initialsOf(firstName, lastName));
</script>

<!--
	aria-hidden, always. The avatar never carries the person's identity on
	its own: it sits inside a control that names her in real text (the
	avatar menu's trigger), so announcing two initials as well would only
	repeat a worse version of the same fact.
-->
<span class="avatar" aria-hidden="true">{initials}</span>

<style>
	@layer components {
		.avatar {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			inline-size: var(--avatar-size);
			block-size: var(--avatar-size);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: 50%;
			background-color: var(--color-surface-container-high);
			color: var(--color-on-surface);
			font-family: var(--font-family-base);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-semibold);
			letter-spacing: var(--text-label-tracking);
			/* A circle of two letters is the one place in the app where a
			   glyph must not reflow: `line-height: 1` keeps the pair on the
			   optical center whatever the fallback font's metrics are. */
			line-height: 1;
		}
	}
</style>
