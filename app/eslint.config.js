import path from 'node:path';
import js from '@eslint/js';
import stylistic from '@stylistic/eslint-plugin';
import svelte from 'eslint-plugin-svelte';
import unicorn from 'eslint-plugin-unicorn';
import { defineConfig, includeIgnoreFile } from 'eslint/config';
import globals from 'globals';
import ts from 'typescript-eslint';

/*
 * The atoms and molecules that own a raw control element, and the
 * elements every other `.svelte` file must reach through them.
 */
const controlOwners = [
	'src/lib/components/atoms/Button.svelte',
	'src/lib/components/atoms/Link.svelte',
	// A door (#1645) is one anchor holding a name and a line, with the
	// name alone as its accessible name -- a shape `Link`'s one label
	// cannot draw. It is the second atom that owns a raw <a>.
	'src/lib/components/atoms/DoorLink.svelte',
	'src/lib/components/atoms/Select.svelte',
	'src/lib/components/atoms/Textarea.svelte',
	'src/lib/components/atoms/TextInput.svelte',
	'src/lib/components/atoms/Checkbox.svelte',
	'src/lib/components/molecules/RadioGroup.svelte'
];

const controlRestrictions = [
	{
		elements: ['button'],
		message: 'Use the Button atom (#lib/components/atoms/Button.svelte) instead of a raw <button> element.'
	},
	{
		elements: ['a'],
		message: 'Use the Link atom (#lib/components/atoms/Link.svelte) instead of a raw <a> element.'
	},
	{
		elements: ['select'],
		message: 'Use the Select atom (#lib/components/atoms/Select.svelte) instead of a raw <select> element.'
	},
	{
		elements: ['textarea'],
		message: 'Use the Textarea atom (#lib/components/atoms/Textarea.svelte) instead of a raw <textarea> element.'
	},
	{
		elements: ['input'],
		message:
			'Use the TextInput atom (#lib/components/atoms/TextInput.svelte), the Checkbox atom, or the RadioGroup molecule instead of a raw <input> element. A file input has no atom -- disable this rule on that one line with a comment saying so (#492).'
	}
];

// The molecules and Templates that own a raw <label> or <fieldset> (#2017).
const fieldOwners = [
	'src/lib/components/molecules/LabeledField.svelte',
	'src/lib/components/molecules/DateFields.svelte',
	'src/lib/components/molecules/CheckboxGroup.svelte',
	'src/lib/components/molecules/RadioGroup.svelte',
	'src/lib/components/templates/QuestionPage.svelte',
	'src/lib/components/templates/FormPage.svelte'
];

const fieldRestrictions = [
	{
		elements: ['label'],
		message:
			'Use LabeledField (#lib/components/molecules/LabeledField.svelte) instead of a raw <label>: it joins the label, hint and error to the control.'
	},
	{
		elements: ['fieldset'],
		message:
			'Use a group molecule -- CheckboxGroup, RadioGroup, DateFields -- or a Template (FormPage, QuestionPage) instead of a raw <fieldset>.'
	}
];

const gitignorePath = path.resolve(import.meta.dirname, '.gitignore');

/*
 * #2018: the type scale reaches a heading only through the `Heading` atom,
 * so a raw heading renders at the browser's own size. The two Templates
 * own a heading `Heading` cannot draw: `QuestionPage`'s <h1> sits inside a
 * <legend> or wraps a <label>, and `OverviewHub`'s is the `display` step
 * the atom deliberately does not offer. The style guide's and the demo's
 * own page chrome (a page title and one heading per state) is not product
 * UI, and every one of its pages would otherwise have to wrap its own
 * outline in the thing it is documenting; the scaffolded Vitest example is
 * not product UI either.
 */
const headingOwners = [
	'src/lib/components/atoms/Heading.svelte',
	'src/lib/components/templates/QuestionPage.svelte',
	'src/lib/components/templates/OverviewHub.svelte',
	'src/routes/style-guide/**',
	'src/routes/demo/**',
	'src/lib/vitest-examples/**'
];

const headingRestrictions = [
	{
		elements: ['h1', 'h2', 'h3', 'h4', 'h5', 'h6'],
		message:
			'Use the Heading atom (#lib/components/atoms/Heading.svelte) instead of a raw heading, so it is on the type scale (#2018). A heading that must take focus when it appears passes `focusOnAppearing`.'
	}
];

/*
 * `svelte/no-restricted-html-elements` is one rule, and a later flat-config
 * block that matches a file replaces an earlier block's options for it
 * rather than adding to them. An `ignores` entry exempts a file from every
 * element in the block, and owning <label> is no reason for LabeledField to
 * hold a raw <button> (#2017). So every `.svelte` file gets every group
 * below, and each owner gets a block of its own holding every group it
 * does not own -- RadioGroup owns controls and fields but still answers to
 * headings, QuestionPage owns fields and headings but still answers to
 * controls. The owner patterns never overlap one another, so the order of
 * those blocks does not matter.
 */
const restrictionGroups = [
	{ owners: controlOwners, restrictions: controlRestrictions },
	{ owners: fieldOwners, restrictions: fieldRestrictions },
	{ owners: headingOwners, restrictions: headingRestrictions }
];

const everyRestriction = restrictionGroups.flatMap((group) => group.restrictions);

const ownerRestrictionBlocks = [...new Set(restrictionGroups.flatMap((group) => group.owners))].map(
	(pattern) => ({
		files: [pattern],
		rules: {
			'svelte/no-restricted-html-elements': [
				'error',
				...restrictionGroups
					.filter((group) => !group.owners.includes(pattern))
					.flatMap((group) => group.restrictions)
			]
		}
	})
);

export default defineConfig(
	includeIgnoreFile(gitignorePath),
	js.configs.recommended,
	ts.configs.recommended,
	svelte.configs.recommended,
	unicorn.configs.recommended,
	{
		languageOptions: { globals: { ...globals.browser, ...globals.node } },
		rules: {
			// typescript-eslint strongly recommend that you do not use the no-undef lint rule on TypeScript projects.
			// see: https://typescript-eslint.io/troubleshooting/faqs/eslint/#i-get-errors-from-the-no-undef-rule-about-global-variables-not-being-defined-even-though-there-are-no-typescript-errors
			'no-undef': 'off',
			// This project mixes camelCase, kebab-case, and PascalCase (for Svelte components) by directory.
			'unicorn/filename-case': ['error', { cases: { camelCase: true, pascalCase: true, kebabCase: true } }]
		}
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: {
				projectService: true,
				extraFileExtensions: ['.svelte'],
				parser: ts.parser
			}
		},
		rules: {
			// Svelte 5 runes mutate top-level `$state` from onMount/event handlers by design.
			'unicorn/no-top-level-assignment-in-function': 'off'
		}
	},
	{
		// SvelteKit route filenames (+page.svelte, +page.server.ts, [param]/) are framework-mandated.
		files: ['src/routes/**'],
		rules: {
			'unicorn/filename-case': 'off'
		}
	},
	{
		/*
		 * A style-guide page is mounted two ways. The router mounts it with
		 * the props it gives every page. The continuum check and the drag
		 * surface mount it as a plain component, with the props that select
		 * one of the states the page declares as `variants` (#1638,
		 * ADR-0025) -- `isEmpty`, `hasError`. Those are not props the router
		 * can supply, which is what this rule guards against everywhere
		 * else, so it is off for these pages and for no other route.
		 */
		files: ['src/routes/style-guide/*/+page.svelte'],
		rules: {
			'svelte/valid-prop-names-in-kit-pages': 'off'
		}
	},
	{
		/*
		 * `app/scripts/**` is the only source in this package that Bun itself
		 * runs (`bun scripts/sync-icons.ts`, `bun run dev:full`), so it is the
		 * only source that must reach for Bun's APIs rather than Node's.
		 *
		 * Everything else is deliberately outside this rule, and the reason is
		 * the runtime rather than a preference. Vitest spawns its workers on
		 * Node even when the run is started with `bunx` -- a probe inside a
		 * spec reports `Bun=undefined versions.bun=none` -- so a spec that
		 * imported `Bun.file` would fail at run time. The e2e suite runs under
		 * Playwright, and vite.config.ts under Vite; both are Node too.
		 */
		files: ['scripts/**/*.ts'],
		rules: {
			'no-restricted-imports': [
				'error',
				{
					patterns: [
						{
							group: ['node:*'],
							message:
								'This file is run by Bun. Use a Bun API instead of a node: import -- Bun.write (which creates parent directories, so no mkdir), Bun.file, Bun.spawn, Bun.$. Specs, e2e and config files run on Node and are outside this rule.'
						}
					]
				}
			]
		}
	},
	{
		// Every Svelte file: every restriction. The owners' own blocks
		// follow and replace this for them (see `restrictionGroups`).
		files: ['**/*.svelte'],
		rules: {
			'svelte/no-restricted-html-elements': ['error', ...everyRestriction]
		}
	},
	...ownerRestrictionBlocks,
	{
		// #1232: app/ is single-quoted (see docs/testing/gates.md's "Formatting"
		// section for the decision and why indentation is gated by
		// indent.usage.spec.ts instead of an ESLint rule). Icon/generated/
		// is machine-written by scripts/sync-icons.ts from raw SVG markup,
		// which is why its strings are double-quoted -- escaping the
		// SVG's own double-quoted attributes would be noisier than the
		// generator's own output, and the file says "Do not edit by
		// hand" for the same reason a human would not hand-fix this.
		files: ['**/*.{ts,js,svelte}'],
		ignores: ['src/lib/components/atoms/Icon/generated/**'],
		plugins: { '@stylistic': stylistic },
		rules: {
			'@stylistic/quotes': ['error', 'single', { avoidEscape: true, allowTemplateLiterals: 'always' }]
		}
	}
);
