import { initializeApp, getApps, getApp, type FirebaseApp } from 'firebase/app';
import { getAuth, connectAuthEmulator, type Auth } from 'firebase/auth';

// Not a secret -- Identity Platform's browser API key only identifies
// which GCP project to talk to, the same way it's public in every
// Firebase web app. The real access boundary is server-side token
// verification (api/internal/authn), not this key.
const firebaseConfig = {
	apiKey: 'browser-key-not-a-secret',
	projectId: 'doula-cloud',
	authDomain: 'doula-cloud.firebaseapp.com'
};

/** Builds the emulator URL connectAuthEmulator expects, or null to use real Identity Platform. */
export function emulatorURL(host: string | undefined): string | null {
	return host ? `http://${host}` : null;
}

/* v8 ignore start -- requires the Firebase SDK's live app/auth wiring, exercised by Playwright e2e not Vitest */
export function getFirebaseAuth(): Auth {
	const app: FirebaseApp = getApps().length ? getApp() : initializeApp(firebaseConfig);
	const auth = getAuth(app);
	const url = emulatorURL(import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_HOST as string | undefined);
	if (url) {
		connectAuthEmulator(auth, url, { disableWarnings: true });
	}
	return auth;
}
/* v8 ignore stop */
