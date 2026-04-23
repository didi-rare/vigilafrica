# Spec: Frontend Component Testing (chore-frontend-tests)

## Context
The React frontend requires a testing strategy to ensure reliability. Since we are using Vite, `vitest` is the natural choice over Jest, as it shares the same configuration and pipeline. We will pair it with `@testing-library/react` to test components in a DOM-like environment.

## Components to Touch
1. `web/package.json` (add devDependencies and test scripts)
2. `web/vite.config.ts` (configure test environment)
3. `web/src/components/EventsDashboard.test.tsx` (new test file)
4. `web/src/api/events.test.ts` (new test file for data hooks)

## Implementation Plan
1. **Dependencies:** Install `vitest`, `jsdom`, `@testing-library/react`, and `@testing-library/jest-dom` in the `web/` directory.
2. **Configuration:** Add `test: { environment: 'jsdom', setupFiles: ['./src/setupTests.ts'] }` to `vite.config.ts`. Add a `test` script to `package.json`.
3. **Setup File:** Create `src/setupTests.ts` to import `@testing-library/jest-dom` for custom matchers.
4. **Test Cases:**
    *   **EventsDashboard:** Mock the API response using `vi.mock` or MSW. Verify that the correct events are rendered and that the staleness/EONET error banner appears when the mock health endpoint reports degraded status.
    *   **API Hooks:** Test that the data fetching logic correctly parses parameters.

## Acceptance Criteria
- [ ] `npm run test` executes successfully in the `web/` directory.
- [ ] At least one UI component (`EventsDashboard`) has rendering and conditional logic tests.
- [ ] The CI pipeline runs frontend tests alongside Go tests.

## Verification Plan
1. Execute `npm run test` locally and verify passing output.
2. Verify GitHub Actions executes the frontend test suite successfully.
