import { onRequest } from 'firebase-functions/v2/https';
import admin from 'firebase-admin';

admin.initializeApp();

// --- Shorten URL Function ---
export const shortenUrl = onRequest(async (req, res) => {
  const { handleShortenUrl } = await import('./shorten-url.js');
  await handleShortenUrl(req, res);
});

export const redirectUrl = onRequest(async (req, res) => {
  const { handleRedirectUrl } = await import('./redirect-url.js');
  await handleRedirectUrl(req, res);
});
