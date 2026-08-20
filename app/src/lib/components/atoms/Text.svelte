<script lang="ts">
	interface Properties {
		text: string;
		size?: 'sm' | 'base' | 'lg';
		muted?: boolean;
	}

	let { text, size = 'base', muted = false }: Properties = $props();

	const textClass = $derived(`size-${size}${muted ? ' muted' : ''}`);
</script>

<p class={textClass}>{text}</p>

<style>
	@layer components {
		p {
			margin: 0;
			font-family: var(--font-family-base);
			color: var(--color-text);
			line-height: 1.5;
		}

		p.size-sm {
			font-size: var(--text-sm);
		}

		p.size-base {
			font-size: var(--text-base);
		}

		p.size-lg {
			font-size: var(--text-lg);
		}

		p.muted {
			color: var(--color-muted);
		}
	}
</style>
