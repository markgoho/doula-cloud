// Icon names synced from phosphor-icons/core (issue #96). `bun run sync-icons`
// pulls both `duotone` and `light` weights for every entry here into
// ./generated -- Icon.svelte picks the weight at render time by size (see
// its size threshold).
export const iconManifest = [
	'check',
	'x',
	'warning',
	'info',
	'arrow-right',
	'arrow-left',
	'arrow-square-out',
	'minus-circle',
	'paperclip',
	'users',
	'receipt',
	'tag',
	'user-check',
	'clipboard-text',
	'file-text',
	'credit-card',
	'caret-down',
	'list',
	'eye',
	'eye-slash'
] as const;

export type IconName = (typeof iconManifest)[number];
