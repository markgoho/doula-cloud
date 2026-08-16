import path from 'node:path';
import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import unicorn from 'eslint-plugin-unicorn';
import { defineConfig, includeIgnoreFile } from 'eslint/config';
import globals from 'globals';
import ts from 'typescript-eslint';

const gitignorePath = path.resolve(import.meta.dirname, '.gitignore');

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
			"no-undef": 'off',
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
		// Override or add rule settings here, such as:
		// 'svelte/button-has-type': 'error'
		rules: {}
	}
);
