# Proposal: Frontend Component Testing (chore-frontend-tests)

## Why
The React frontend in `web/` currently has 0% test coverage and lacks any testing framework in its `package.json`. As we move toward a stable v1.0 launch, we need automated validation to ensure that critical UI paths—like rendering the dashboard, displaying the staleness/error banner, and map initialization—do not regress during future development.

## What Changes
We will introduce `vitest`, `jsdom`, and `@testing-library/react` to the frontend workspace. This will provide a fast, modern testing environment compatible with our Vite build setup.

## Out of Scope
- End-to-End (E2E) testing with Playwright/Cypress (this can be considered later).
- 100% line coverage. The goal is to cover the critical interaction paths, not to hit arbitrary coverage metrics.

## User Impact
No visible changes for end-users. For maintainers, it prevents shipping broken UI states and ensures the data fetching hooks integrate properly with the React lifecycle.
