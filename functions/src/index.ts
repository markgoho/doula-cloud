import { onRequest, type Request } from 'firebase-functions/v2/https';
import { type Response } from 'express';
import validUrl from 'valid-url';
import { nanoid } from 'nanoid';
import admin from 'firebase-admin';

admin.initializeApp();
const db = admin.firestore();

const allowedOrigins = ['*']; // Replace with specific origins for security

/**
 * Sets appropriate CORS headers based on the request origin.
 */
const setCorsHeaders = (req: Request, res: Response) => {
  const origin = req.headers.origin as string; // Cast origin as string
  if (
    allowedOrigins.includes('*') ||
    (origin && allowedOrigins.includes(origin))
  ) {
    // If '*' is allowed OR the specific origin is allowed
    res.set('Access-Control-Allow-Origin', origin || '*'); // Send back specific origin or '*'
  }
  // If origin is not in allowedOrigins and '*' is not allowed, no header is set,
  // and the browser will block the request.
};

// --- Shorten URL Function ---
export const shortenUrl = onRequest(async (req, res) => {
  // Handle CORS Preflight request (OPTIONS method)
  if (req.method === 'OPTIONS') {
    setCorsHeaders(req, res);
    res.set('Access-Control-Allow-Methods', 'POST, GET, OPTIONS'); // Specify allowed methods
    res.set('Access-Control-Allow-Headers', 'Content-Type, Authorization'); // Specify allowed headers
    res.set('Access-Control-Max-Age', '3600'); // Cache preflight response for 1 hour
    res.status(204).send('');
    return; // End execution for OPTIONS request
  }

  // Set CORS headers for the actual request (e.g., POST)
  // Done before sending the actual response or error
  setCorsHeaders(req, res);

  // --- Main Logic (await can be used directly here) ---
  if (req.method !== 'POST') {
    res.status(405).send('Method Not Allowed');
    return;
  }
  if (req.headers['content-type'] !== 'application/json') {
    res
      .status(400)
      .send({ error: 'Bad Request: Content-Type must be application/json' });
    return;
  }

  // *** ADD AUTHENTICATION CHECK HERE ***
  console.warn('!!! Authentication NOT IMPLEMENTED for shortenUrl !!!');
  // Example: const decodedToken = await verifyAuthToken(req); if (!decodedToken) { res.status(403)...; return; }

  const { longUrl } = req.body as { longUrl: string };
  if (!longUrl || !validUrl.isWebUri(longUrl)) {
    res.status(400).send({ error: 'Invalid or missing URL' });
    return;
  }

  try {
    let shortCode = '';
    let retries = 0;
    const maxRetries = 5;
    let exists = true;

    while (exists && retries < maxRetries) {
      shortCode = nanoid(7);
      // Direct await is now fine
      const doc = await db.collection('urls').doc(shortCode).get();
      exists = doc.exists;
      retries++;
    }

    if (exists) {
      res.status(500).send({ error: 'Failed to generate unique short code.' });
      return;
    }

    const urlData = {
      longUrl,
      createdAt: admin.firestore.FieldValue.serverTimestamp(),
      clicks: 0,
      // userId: decodedToken.uid, // If auth is implemented
    };
    // Direct await is now fine
    await db.collection('urls').doc(shortCode).set(urlData);

    const shortUrlResult = `https://dou.la/${shortCode}`;
    res.status(201).send({ shortUrl: shortUrlResult });
  } catch (error) {
    console.error('Error shortening URL:', error);
    // CORS header was already set earlier
    res.status(500).send({ error: 'Internal server error' });
  }
});

export const redirectUrl = onRequest(async (req, res) => {
  // Handle CORS Preflight (OPTIONS) - May not be strictly needed for 3xx redirects
  // initiated by browser navigation, but good practice if the endpoint *could*
  // be fetched by JS (e.g., to check validity before redirecting).
  if (req.method === 'OPTIONS') {
    setCorsHeaders(req, res);
    res.set('Access-Control-Allow-Methods', 'GET, OPTIONS');
    res.set('Access-Control-Allow-Headers', 'Authorization'); // If ever needed
    res.set('Access-Control-Max-Age', '3600');
    res.status(204).send('');
    return;
  }
  // Set CORS for actual request - again, less critical for pure redirects
  setCorsHeaders(req, res);

  // --- Main Logic (await can be used directly here) ---
  const shortCode = req.path.substring(1); // Get path after the leading '/'

  if (!shortCode) {
    res.redirect(302, 'https://doula.cloud'); // Example redirect for root
    return;
  }

  try {
    const docRef = db.collection('urls').doc(shortCode);
    // Direct await is fine
    const doc = await docRef.get();

    if (!doc.exists) {
      res.status(404).send('Short URL not found.');
      return;
    }

    const { longUrl } = doc.data() as { longUrl: string };

    // Non-blocking update
    docRef
      .update({
        clicks: admin.firestore.FieldValue.increment(1),
      })
      .catch((err: unknown) => {
        console.error('Error incrementing clicks:', shortCode, err);
      });

    res.redirect(301, longUrl);
  } catch (error) {
    console.error('Error redirecting URL:', shortCode, error);
    // CORS header was already set
    res.status(500).send('Internal Server Error.');
  }
});
