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
	{ name: 'Birth plan view', slug: 'birth-plan-view' },
	{ name: 'Contract form', slug: 'contract-form' },
	{ name: 'Contract status', slug: 'contract-status' },
	{ name: 'Contract view', slug: 'contract-view' },
	{ name: 'Description list', slug: 'description-list' },
	{ name: 'Labeled field', slug: 'labeled-field' },
	{ name: 'MembershipFields', slug: 'membership-fields' },
	{ name: 'Radio group', slug: 'radio-group' },
	{ name: 'Sign out button', slug: 'sign-out-button' },
	{ name: 'Work state field', slug: 'work-state-field' }
] as const;

export const organismPages = [
	{ name: 'Contract template editor', slug: 'contract-template-editor' },
	{ name: 'Data table', slug: 'data-table' },
	{ name: 'Dynamic field editor', slug: 'dynamic-field-editor' },
	{ name: 'Invoice section', slug: 'invoice-section' },
	{ name: 'Message thread', slug: 'message-thread' },
	{ name: 'Offer inbox', slug: 'offer-inbox' },
	{ name: 'Offer section', slug: 'offer-section' },
	{ name: 'Plan instance form', slug: 'plan-instance-form' },
	{ name: 'Sign contract', slug: 'sign-contract' }
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
