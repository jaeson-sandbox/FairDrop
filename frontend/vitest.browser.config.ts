/// <reference types="vitest/config" />
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import {playwright} from '@vitest/browser-playwright'

/*
  A second, standalone Vitest project: real Chromium via vitest browser mode
  (the Playwright provider), proving the handful of rules jsdom cannot
  evaluate at all -- 320px reflow, 200% text, the 44px target floor, and
  forced-colors (D-065, D-068). This is deliberately its own file rather than
  a `vitest.config.ts` (which Vitest auto-discovers ahead of `vite.config.ts`
  for a bare `vitest` invocation -- naming it that would silently swap out the
  jsdom project) or a `test.projects` entry inside vite.config.ts (which a
  bare `vitest run` would also pick up, pulling a browser into the one
  command the spec requires to keep working unchanged). `npm run test:browser`
  -- `vitest run --config vitest.browser.config.ts` -- is the only door in.

  Its `test.include` points at frontend/browser/, never at src/, so nothing
  here can be picked up by the jsdom project's "src/**" test glob by accident
  either.
*/
export default defineConfig({
    plugins: [tailwindcss(), react()],
    test: {
        include: ['browser/**/*.test.{ts,tsx}'],
        browser: {
            enabled: true,
            // Explicit rather than left to the `process.env.CI` default: a
            // developer running this locally should get the same headless
            // Chromium CI does, not a window that pops up and blocks on a
            // machine with no attached display.
            headless: true,
            // The one screenshot this suite keeps is the explicit forced-colors
            // QR capture below, named and placed on purpose. An auto-captured
            // failure screenshot is debugging output, not retained evidence --
            // off by default so a red run cannot leave a stray file for a
            // later `git status` to puzzle over.
            screenshotFailures: false,
            provider: playwright(),
            instances: [{browser: 'chromium'}],
        },
    },
})
