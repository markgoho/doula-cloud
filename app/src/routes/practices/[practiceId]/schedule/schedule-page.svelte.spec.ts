import { vi } from 'vitest';
import { describeReadFailure } from '#lib/readFailureSpec.js';

vi.mock('$app/state', async () => {
	const { stateModule } = await import('#lib/readFailureSpec.js');
	return stateModule;
});
vi.mock('./ScheduleScreen.svelte', async () => import('#lib/TestRetry.svelte'));
vi.mock('$app/navigation', async () => {
	const { navigationModule } = await import('#lib/readFailureSpec.js');
	return navigationModule;
});

describeReadFailure({
	title: 'the Schedule page',
	ticket: '#2747',
	path: 'schedule',
	loadPage: () => import('./+page.svelte'),
	invalidates: 'app:schedule'
});

vi.mock('#lib/api.js', async (importOriginal) => {
	const { apiModule } = await import('#lib/readFailureSpec.js');
	return apiModule(importOriginal);
});
