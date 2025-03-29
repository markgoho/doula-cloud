import { type Request } from 'firebase-functions/v2/https';
import { type Response } from 'express';
import { setCorsHeaders } from './util/set-cors-headers';
import admin from 'firebase-admin';

export const handleRedirectUrl = async (req: Request, res: Response) => {
  const db = admin.firestore();
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

    res.redirect(302, longUrl);
  } catch (error) {
    console.error('Error redirecting URL:', shortCode, error);
    // CORS header was already set
    res.status(500).send('Internal Server Error.');
  }
};
