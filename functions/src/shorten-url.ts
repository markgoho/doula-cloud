import { type Request } from 'firebase-functions/v2/https';
import { type Response } from 'express';
import { setCorsHeaders } from './util/set-cors-headers';
import { nanoid } from 'nanoid';
import validUrl from 'valid-url';
import admin from 'firebase-admin';

export const handleShortenUrl = async (req: Request, res: Response) => {
  const db = admin.firestore();

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
};
