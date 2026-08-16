import { defineLayoutPrimitive, type PropDefaults } from './defineLayoutPrimitive.js';

interface PrimitiveSpec<Defaults extends PropDefaults> {
	tagName: string;
	defaults: Defaults;
	css: (values: Defaults, selector: string) => string;
}

/**
 * The 12 Every Layout-inspired primitives whose non-default styling is a
 * pure function of their resolved config -- see ./icon.ts for the one
 * primitive (Icon's `label`) that needs ARIA attribute reflection instead of
 * CSS injection, and so isn't expressible as one of these.
 */
export const primitiveSpecs: PrimitiveSpec<PropDefaults>[] = [
	{
		tagName: 'stack-l',
		defaults: { space: 'var(--space-4)' },
		css: (v, s) => `${s} > * + * { margin-block-start: ${v.space}; }`
	},
	{
		tagName: 'box-l',
		defaults: { padding: 'var(--space-4)', 'border-width': 'var(--border-thin)' },
		css: (v, s) => `${s} { padding: ${v.padding}; border-width: ${v['border-width']}; }`
	},
	{
		tagName: 'center-l',
		defaults: { max: 'var(--measure)', gutters: '0' },
		css: (v, s) => `${s} { max-inline-size: ${v.max}; padding-inline: ${v.gutters}; }`
	},
	{
		tagName: 'cluster-l',
		defaults: { space: 'var(--space-4)', justify: 'flex-start', align: 'flex-start' },
		css: (v, s) =>
			`${s} { gap: ${v.space}; justify-content: ${v.justify}; align-items: ${v.align}; }`
	},
	{
		tagName: 'sidebar-l',
		defaults: { space: 'var(--space-4)', 'content-min': '50%' },
		css: (v, s) => `${s} { gap: ${v.space}; }\n${s} > :last-child { min-inline-size: ${v['content-min']}; }`
	},
	{
		tagName: 'switcher-l',
		defaults: { threshold: 'var(--measure)', space: 'var(--space-4)', limit: '4' },
		css: (v, s) => {
			const wrapAt = `n + ${Number(v.limit) + 1}`;
			return (
				`${s} { gap: ${v.space}; }\n` +
				`${s} > * { flex-basis: calc((${v.threshold} - 100%) * 999); }\n` +
				`${s} > :nth-last-child(${wrapAt}),\n${s} > :nth-last-child(${wrapAt}) ~ * { flex-basis: 100%; }`
			);
		}
	},
	{
		tagName: 'cover-l',
		defaults: { space: 'var(--space-4)', 'min-height': '100vh', centered: 'h1' },
		css: (v, s) =>
			`${s} { min-block-size: ${v['min-height']}; }\n` +
			// :not([no-pad]) outranks primitives.css's `cover-l[no-pad] { padding: 0; }`
			// on specificity regardless of injection order, so no-pad still wins
			// when a non-default space is also set.
			`${s}:not([no-pad]) { padding: ${v.space}; }\n` +
			`${s} > * { margin-block: ${v.space}; }\n` +
			`${s} > :first-child:not(${v.centered}) { margin-block-start: 0; }\n` +
			`${s} > :last-child:not(${v.centered}) { margin-block-end: 0; }\n` +
			`${s} > ${v.centered} { margin-block: auto; }`
	},
	{
		tagName: 'grid-l',
		defaults: { min: '16rem', space: 'var(--space-4)' },
		css: (v, s) =>
			`${s} { gap: ${v.space}; grid-template-columns: repeat(auto-fit, minmax(min(${v.min}, 100%), 1fr)); }`
	},
	{
		tagName: 'frame-l',
		defaults: { ratio: '16 / 9' },
		css: (v, s) => `${s} { aspect-ratio: ${v.ratio}; }`
	},
	{
		tagName: 'reel-l',
		defaults: { space: 'var(--space-2)', 'item-width': 'auto', height: 'auto' },
		css: (v, s) =>
			`${s} { block-size: ${v.height}; }\n` +
			`${s} > * { flex-basis: ${v['item-width']}; }\n` +
			`${s} > * + * { margin-inline-start: ${v.space}; }`
	},
	{
		tagName: 'imposter-l',
		defaults: { margin: '0px' },
		css: (v, s) =>
			`${s} { max-inline-size: calc(100% - (${v.margin} * 2)); max-block-size: calc(100% - (${v.margin} * 2)); }`
	},
	{
		tagName: 'container-l',
		defaults: { name: '' },
		css: (v, s) => `${s} { container-name: ${v.name}; }`
	}
];

export function registerDataPrimitives(): void {
	for (const spec of primitiveSpecs) {
		defineLayoutPrimitive(spec.tagName, spec.defaults, spec.css);
	}
}
