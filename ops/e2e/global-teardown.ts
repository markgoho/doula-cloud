import { stopStack } from '../../app/e2e/stack';

export default function globalTeardown() {
	stopStack();
}
