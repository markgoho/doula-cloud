import { vi } from 'vitest';
import { describeReadFailure } from '#lib/readFailureSpec.js';

vi.mock('$app/state', async () => {
	const { stateModule } = await import('#lib/readFailureSpec.js');
	return stateModule;
});
vi.mock('./InvoicesScreen.svelte', async () => import('#lib/TestRetry.svelte'));
vi.mock('$app/navigation', async () => {
	const { navigationModule } = await import('#lib/readFailureSpec.js');
	return navigationModule;
});

describeReadFailure({
	title: 'the Invoices page',
	ticket: '#2746',
	path: 'invoices',
	loadPage: () => import('./+page.svelte'),
	invalidates: 'app:invoices'
});

vi.mock('#lib/api.js', async (importOriginal) => {
	const { apiModule } = await import('#lib/readFailureSpec.js');
	return apiModule(importOriginal);
});
