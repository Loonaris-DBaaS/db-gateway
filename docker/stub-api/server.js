const express = require('express');
const routes = require('./routes.json');

const app = express();
const PORT = process.env.PORT || 3001;

app.get('/internal/routes/:keyHash', (req, res) => {
  const entry = routes[req.params.keyHash];
  if (!entry) {
    return res.status(404).json({ error: 'not found' });
  }

  const auth = req.headers.authorization;
  if (!auth || !auth.startsWith('Bearer ')) {
    return res.status(401).json({ error: 'unauthorized' });
  }

  console.log(`[stub-api] GET /internal/routes/${req.params.keyHash} -> tenant_id=${entry.tenant_id}`);
  res.json(entry);
});

app.get('/health', (_req, res) => {
  res.json({ status: 'ok' });
});

app.listen(PORT, () => {
  console.log(`Stub control plane API listening on :${PORT}`);
  console.log(`Loaded ${Object.keys(routes).length} tenant routes`);
});