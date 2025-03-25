import { onRequest } from 'firebase-functions/v2/https';
import express from 'express';
import cors from 'cors';

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
