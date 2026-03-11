import path from 'path'
import { defineConfig, devices } from '@playwright/test'

// The web UI is served by Vite in test mode (port 5173).
// SANGRAHA_ADMIN_URL is the address the dashboard uses to reach the admin API;
// it defaults to http://localhost:9001 and is intercepted by page.route() mocks
// so a real sangraha binary is not required.
const webPort = 5173

export default defineConfig({
  testDir: '.',
  timeout: 60_000,
  retries: 0,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],

  // Auto-start the Vite dev server before running tests.
  webServer: {
    command: 'npm run dev -- --port ' + webPort,
    cwd: path.join(__dirname, '../../web'),
    port: webPort,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },

  use: {
    baseURL: `http://localhost:${webPort}`,
    headless: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
})
