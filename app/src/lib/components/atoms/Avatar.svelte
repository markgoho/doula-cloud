<script module lang="ts">
	/*
	 * Initials are derived here rather than served, because a name is
	 * already on the wire (`GET /api/staff/session`, and the portal's
	 * Engagement detail) and a second field holding two letters of it would
	 * be a copy that can go stale.
	 *
	 * First and last word only: a middle name adds a letter nobody reads at
	 * 34px, and three letters no longer fit the circle. Exported so the
	 * style guide and the specs use the same rule as the component.
	 */
	export function initialsOf(name: string): string {
		const words = name.trim().split(/\s+/).filter(Boolean);
		if (words.length === 0) return '';
		const first = words[0]!;
		const last = words.at(-1)!;
		const letters = words.length === 1 ? first.slice(0, 1) : first.slice(0, 1) + last.slice(0, 1);
		return letters.toLocaleUpperCase();
	}
</script>

<script lang="ts">
	interface Properties {
		name: string;
	}

	let { name }: Properties = $props();

	const initials = $derived(initialsOf(name));
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
			   optical centre whatever the fallback font's metrics are. */
			line-height: 1;
		}
	}
</style>
