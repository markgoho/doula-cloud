/**
 * Import function triggers from their respective submodules:
 *
 * import {onCall} from "firebase-functions/v2/https";
 * import {onDocumentWritten} from "firebase-functions/v2/firestore";
 *
 * See a full list of supported triggers at https://firebase.google.com/docs/functions
 */

import { onRequest } from 'firebase-functions/v2/https';
import express from 'express';
import cors from 'cors';

// Start writing functions
// https://firebase.google.com/docs/functions/typescript

const app = express();
app.use(cors({ origin: true }));

app.get('/hello', (req, res) => {
  res.send('Hello World!');
});

app.get('/:shortCode', (req, res) => {
  res.send(
    `Hello from the redirect endpoint! Short code: ${req.params.shortCode}`
  );
});

export const api = onRequest(app);
