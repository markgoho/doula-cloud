<script lang="ts">
	interface Properties {
		level: 1 | 2 | 3 | 4 | 5 | 6;
		text: string;
	}

	let { level, text }: Properties = $props();

	const tag = $derived(`h${level}`);
</script>

<svelte:element this={tag} class={`level-${level}`}>{text}</svelte:element>

<style>
	@layer components {
		.level-1,
		.level-2,
		.level-3,
		.level-4,
		.level-5,
		.level-6 {
			margin: 0;
			font-family: var(--font-family-base);
			color: var(--color-text);
			line-height: 1.2;
			font-weight: var(--font-weight-semibold);
		}

		.level-1 {
			font-size: var(--text-3xl);
		}

		.level-2 {
			font-size: var(--text-2xl);
		}

		.level-3 {
			font-size: var(--text-xl);
		}

		.level-4 {
			font-size: var(--text-lg);
		}

		.level-5 {
			font-size: var(--text-base);
		}

		/* Same size as Text's "sm", so weight/case/tracking carry the
		   hierarchy instead of size alone at this end of the scale. */
		.level-6 {
			font-size: var(--text-sm);
			text-transform: uppercase;
			letter-spacing: 0.05em;
		}
	}
</style>
