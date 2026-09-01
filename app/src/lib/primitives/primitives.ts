import { defineLayoutPrimitive, type PropertyDefaults } from './defineLayoutPrimitive.js';

interface PrimitiveSpec<Defaults extends PropertyDefaults> {
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
export const primitiveSpecs: PrimitiveSpec<PropertyDefaults>[] = [
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
		// `side: 'start'` and `basis: '20rem'` reproduce primitives.css's own
		// hardcoded defaults exactly, so an instance at default config still
		// injects nothing (#564) -- `side: 'end'` swaps which DOM child gets
		// the fixed basis and which gets the dominant, wrap-triggering
		// minimum, for a Sidebar whose sidebar is visually and structurally
		// LAST (OverviewHub's secondary) without reordering the DOM.
		defaults: { space: 'var(--space-4)', 'content-min': '50%', basis: '20rem', side: 'start' },
		css: (v, s) => {
			const sidebarChild = v.side === 'end' ? ':last-child' : ':first-child';
			const contentChild = v.side === 'end' ? ':first-child' : ':last-child';
			return (
				`${s} { gap: ${v.space}; }\n` +
				// The sidebar side's own min-inline-size is reset to 0 explicitly,
				// not left unset: primitives.css's own unconditional `:last-child`
				// rule would otherwise leak its 50% through onto this element when
				// `side: 'end'` makes `:last-child` the SIDEBAR rather than the
				// content -- caught by rendering an instance and reading its
				// computed style back, not assumed.
				`${s} > ${sidebarChild} { flex-basis: ${v.basis}; flex-grow: 1; min-inline-size: 0; }\n` +
				`${s} > ${contentChild} { flex-basis: 0; flex-grow: 999; min-inline-size: ${v['content-min']}; }`
			);
		}
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
