import { startStack } from '../../app/e2e/stack';
import { PREVIEW_SERVER_ORIGIN } from '../../app/e2e/ports';

export default async function globalSetup() {
	await startStack(PREVIEW_SERVER_ORIGIN);
}
