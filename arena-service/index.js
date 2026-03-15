const express = require('express');
const app = express();
const port = 3000;

const SERVICE_NAME = process.env.SERVICE_NAME || 'unknown';
const UPSTREAM_URL = process.env.UPSTREAM_URL || '';

console.log(`${SERVICE_NAME} started`);

app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: SERVICE_NAME });
});

app.get('/', (req, res) => {
  res.json({ service: SERVICE_NAME, upstream: UPSTREAM_URL });
});

app.listen(port, () => {
  console.log(`${SERVICE_NAME} listening on port ${port}`);
});
