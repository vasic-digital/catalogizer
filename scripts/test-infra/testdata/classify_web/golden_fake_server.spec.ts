import { test } from '@playwright/test';
import http from 'node:http';
test('local fake backend', async ({ page }) => {
  const srv = http.createServer((req, res) => { res.end('{}'); }).listen(0);
  await page.goto('/');
  srv.close();
});
