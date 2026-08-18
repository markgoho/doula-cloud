import { startStack } from './stack';
import { PREVIEW_SERVER_ORIGIN } from './ports';

export default async function globalSetup() {
	await startStack(PREVIEW_SERVER_ORIGIN);
}
