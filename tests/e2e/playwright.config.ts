import { defineConfig } from '@playwright/test';

const PORT = Number(process.env.HITIME_PORT ?? 9012);
// Set HITIME_BASE_PATH=/hitime to run the whole suite against the app mounted
// under a URL prefix (as it is behind the gportal reverse proxy).
const BASE_PATH = (process.env.HITIME_BASE_PATH ?? '').replace(/^\/*|\/*$/g, '');
export const BASE_URL = `http://localhost:${PORT}${BASE_PATH ? `/${BASE_PATH}/` : ''}`;

export default defineConfig({
  testDir: './specs',
  // Peers launch their own browsers and the signalling server is process-global
  // state, so parallel files would interfere with room/user assertions.
  workers: 1,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  timeout: 90_000,
  expect: { timeout: 30_000 },
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],
  use: {
    baseURL: BASE_URL,
    trace: 'retain-on-failure',
  },
  webServer: {
    command: `go run . --dev --port ${PORT} --base-path "${BASE_PATH}"`,
    cwd: '../..',
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
