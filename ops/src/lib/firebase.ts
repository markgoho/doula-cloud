import { getApp, getApps, initializeApp } from 'firebase/app';
import {
	connectAuthEmulator,
	getAuth,
	getMultiFactorResolver,
	initializeAuth,
	inMemoryPersistence,
	signInWithEmailAndPassword,
	signOut,
	TotpMultiFactorGenerator,
	type Auth,
	type MultiFactorError,
	type UserCredential
} from 'firebase/auth';

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

/**
 * Signs in again with the password and an authenticator-app code, and
 * hands back the fresh ID token for doula-ops's step-up. Identity
 * Platform has no code-only re-authentication, so the password comes
 * first. Throws Identity Platform's own refusal.
 */
export async function freshIdToken(email: string, password: string, code: string): Promise<string> {
	const auth = getOperatorAuth();
	let credential: UserCredential;
	try {
		credential = await signInWithEmailAndPassword(auth, email, password);
	} catch (error) {
		if ((error as { code?: unknown }).code !== 'auth/multi-factor-auth-required') throw error;
		const resolver = getMultiFactorResolver(auth, error as MultiFactorError);
		credential = await resolver.resolveSignIn(TotpMultiFactorGenerator.assertionForSignIn(resolver.hints[0].uid, code));
	}
	try {
		return await credential.user.getIdToken();
	} finally {
		await signOut(auth);
	}
}
/* v8 ignore stop */
