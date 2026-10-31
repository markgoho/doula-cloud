import { getApp, getApps, initializeApp } from 'firebase/app';
import { connectAuthEmulator, getAuth, initializeAuth, inMemoryPersistence, type Auth } from 'firebase/auth';

// The doula-cloud web app's browser key, the same public key app/ uses:
// it names the project, and token verification in doula-ops is the
// boundary. The operator tenant is chosen at build time.
const firebaseConfig = {
	apiKey: 'AIzaSyABnAc22teViyKS0EzLmp1Gcxi-uQuU5UE',
	projectId: 'doula-cloud',
	authDomain: 'doula-cloud.firebaseapp.com'
};

/**
The emulator URL connectAuthEmulator takes, or undefined for Identity Platform.
*/
export function emulatorURL(host: string | undefined): string | undefined {
	return host ? `http://${host}` : undefined;
}

/* v8 ignore start -- the Firebase SDK's live wiring, exercised by ops/'s Playwright suite */
export function getOperatorAuth(): Auth {
	const isFirstUse = getApps().length === 0;
	const app = isFirstUse ? initializeApp(firebaseConfig) : getApp();
	const auth = isFirstUse ? initializeAuth(app, { persistence: inMemoryPersistence }) : getAuth(app);
	if (isFirstUse) {
		// eslint-disable-next-line unicorn/no-null -- the SDK types an unset tenant as null
		auth.tenantId = (import.meta.env.VITE_OPS_TENANT_ID as string | undefined) ?? null;
		const url = emulatorURL(import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_HOST as string | undefined);
		if (url) connectAuthEmulator(auth, url, { disableWarnings: true });
	}
	return auth;
}
/* v8 ignore stop */
