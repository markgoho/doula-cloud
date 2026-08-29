export const atomPages = [
	{ name: 'Badge', slug: 'badge' },
	{ name: 'Button', slug: 'button' },
	{ name: 'Checkbox', slug: 'checkbox' },
	{ name: 'Heading', slug: 'heading' },
	{ name: 'Icon', slug: 'icon' },
	{ name: 'Link', slug: 'link' },
	{ name: 'Notice', slug: 'notice' },
	{ name: 'Select', slug: 'select' },
	{ name: 'Skeleton', slug: 'skeleton' },
	{ name: 'Text', slug: 'text' },
	{ name: 'Text input', slug: 'text-input' }
] as const;

export const moleculePages = [
	{ name: 'Description list', slug: 'description-list' },
	{ name: 'Labeled field', slug: 'labeled-field' },
	{ name: 'MembershipFields', slug: 'membership-fields' },
	{ name: 'Radio group', slug: 'radio-group' },
	{ name: 'Sign out button', slug: 'sign-out-button' }
] as const;

export const organismPages = [
	{ name: 'Data table', slug: 'data-table' },
	{ name: 'Dynamic field editor', slug: 'dynamic-field-editor' },
	{ name: 'Message thread', slug: 'message-thread' }
] as const;

/*
 * Templates own their own gutters and max-width (ADR-0018), so these pages
 * render outside the style-guide's own padded wrapper -- see +layout.svelte.
 */
export const templatePages = [
	{ name: 'Form page', slug: 'form-page' },
	{ name: 'Overview hub', slug: 'overview-hub' },
	{ name: 'Record detail', slug: 'record-detail' }
] as const;

export const templateSlugs: readonly string[] = templatePages.map((templatePage) => templatePage.slug);
