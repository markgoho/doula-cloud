import { type Request } from 'firebase-functions/v2/https';
import { type Response } from 'express';

const allowedOrigins = ['*']; // Replace with specific origins for security

/**
 * Sets appropriate CORS headers based on the request origin.
 */
export const setCorsHeaders = (req: Request, res: Response) => {
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
