# Spec: Developer-Driven API Load Testing (chore-k6-performance-tests)

## Context
To guarantee API stability for v1.0, we need a standardized way for developers to verify performance locally against a seeded database before merging pull requests. We will use `k6` as our load-testing tool because its tests are written in JavaScript and easily version-controlled alongside our code.

## Components to Touch
1. `tests/performance/load-test.js` (New file)
2. `package.json` (Root level - add an npm script to run the test)

## Implementation Plan
1.  **Create Test Directory:** Create a `tests/performance` directory at the project root.
2.  **Write k6 Script:** Create `load-test.js` with the following configuration:
    *   **Options:** Define stages to simulate realistic traffic (e.g., 10s ramp-up to 50 VUs, 30s hold at 50 VUs, 10s ramp-down).
    *   **Thresholds:** Set `http_req_duration: ['p(95)<200']` to explicitly fail the test if 95% of requests take longer than 200ms.
    *   **Scenario:** The VU function should execute a `GET` request to `http://localhost:8080/v1/events`, occasionally appending realistic query parameters (e.g., `?country=NG` or `?category=wildfires`).
3.  **Add Helper Script:** Add `"test:load": "k6 run tests/performance/load-test.js"` to the root `package.json` for convenience.

## Acceptance Criteria
- [ ] `tests/performance/load-test.js` exists and contains valid k6 configuration.
- [ ] The script accurately tests the local API endpoint and enforces a 200ms p95 threshold.
- [ ] Running the script locally against the `demo` Docker compose database yields a pass/fail output.

## Verification Plan
1. Start the API locally (`npm run api:dev` and `docker compose -f docker-compose.demo.yml up -d`).
2. Execute `k6 run tests/performance/load-test.js` (or via docker run if k6 is not installed natively).
3. Verify the k6 summary output displays the p95 duration and successfully enforces the threshold.
