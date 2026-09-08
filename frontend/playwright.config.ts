import { defineConfig } from '@playwright/test'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
const binary = process.env.OSTRM_BINARY || fileURLToPath(new URL('../dist/ostrm_3.0.0-dev_darwin_arm64/ostrm', import.meta.url))
const dir = mkdtempSync(join(tmpdir(), 'ostrm-browser-'))
export default defineConfig({
  testDir: './tests', workers: 1, use: { baseURL: 'http://127.0.0.1:31119', headless: true },
  webServer: { command: `"${binary}" serve --listen 127.0.0.1:31119 --data-dir "${dir}"`, url: 'http://127.0.0.1:31119/health', reuseExistingServer: false },
})
