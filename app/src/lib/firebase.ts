import { initializeApp, getApps, getApp, type FirebaseApp } from 'firebase/app';
import { getAuth, connectAuthEmulator, type Auth } from 'firebase/auth';

// Not a secret -- Identity Platform's browser API key only identifies
// which GCP project to talk to, the same way it's public in every
// Firebase web app. The real access boundary is server-side token
// verification (api/internal/authn), not this key.
//
// It does have to be the project's *real* key, though. This was the
// literal string 'browser-key-not-a-secret', which the auth emulator
// accepts (it validates nothing) and real Identity Platform rejects with
// `auth/api-key-not-valid`. Every e2e test runs against the emulator, so
// nothing caught it, and the deployed app could not log anyone in or sign
// anyone up. Value is `firebase apps:sdkconfig WEB`'s apiKey for the
// doula-cloud web app.
const firebaseConfig = {
	apiKey: 'AIzaSyABnAc22teViyKS0EzLmp1Gcxi-uQuU5UE',
	projectId: 'doula-cloud',
	authDomain: 'doula-cloud.firebaseapp.com'
};

/**
Builds the emulator URL connectAuthEmulator expects, or undefined to use real Identity Platform.
*/
export function emulatorURL(host: string | undefined): string | undefined {
	return host ? `http://${host}` : undefined;
}

/* v8 ignore start -- requires the Firebase SDK's live app/auth wiring, exercised by Playwright e2e not Vitest */
export function getFirebaseAuth(): Auth {
	const app: FirebaseApp = getApps().length > 0 ? getApp() : initializeApp(firebaseConfig);
	const auth = getAuth(app);
	const url = emulatorURL(import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_HOST as string | undefined);
	if (url) {
		connectAuthEmulator(auth, url, { disableWarnings: true });
	}
	return auth;
}
/* v8 ignore stop */
