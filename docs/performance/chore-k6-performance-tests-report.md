# k6 Performance Test Report: chore-k6-performance-tests

Date: 2026-04-24

## Scope

This report covers the developer-driven k6 load test added for the local VigilAfrica API:

- Targets: public read API endpoints, weighted by expected user traffic
- Runner: Docker image `grafana/k6:latest`
- API base URL during verification: `http://host.docker.internal:8080`
- Local demo stack: `docker compose -f docker-compose.demo.yml -f tests/performance/docker-compose.performance.yml up -d`

## Current Test Profile

The committed test script now uses a higher launch-readiness profile:

- Ramp up: 30 seconds to 500 virtual users
- Hold: 1 minute at 500 virtual users
- Ramp down: 30 seconds to 0 virtual users
- Threshold: `http_req_duration: p(95)<200`
- Failure guard: `http_req_failed: rate<0.01`

The profile can be tuned without editing the file:

- `VIGILAFRICA_TARGET_VUS`
- `VIGILAFRICA_RAMP_UP_DURATION`
- `VIGILAFRICA_HOLD_DURATION`
- `VIGILAFRICA_RAMP_DOWN_DURATION`
- `VIGILAFRICA_THINK_TIME_SECONDS`
- `VIGILAFRICA_SETUP_WAIT_SECONDS`
- `VIGILAFRICA_SETUP_POLL_SECONDS`

The request mix exercises realistic dashboard/API traffic:

| Endpoint | Weight | Why |
| --- | ---: | --- |
| `GET /v1/events` | 60% | Main public dashboard read path with filter and pagination variants |
| `GET /v1/context` | 15% | GeoIP/location context plus nearby event lookup |
| `GET /v1/states` | 10% | Country/state filter support |
| `GET /v1/events/{id}` | 10% | Detail view traffic using IDs discovered during setup |
| `GET /v1/enrichment-stats` | 3% | Operational enrichment metrics |
| `GET /health` | 2% | Monitor-style health checks |

`GET /openapi.yaml`, `GET /docs`, and `GET /docs/` are intentionally excluded from load testing because they are local documentation/testing surfaces rather than core product traffic.

Within `GET /v1/events`, the script rotates through:

- Unfiltered events with `limit=50`
- `country=Nigeria`
- `country=Ghana`
- `category=floods`
- `category=wildfires`
- `country=Nigeria&category=floods&status=open`
- Pagination via `limit=25&offset=25`

## Verification Commands

Static checks:

```powershell
npm run spec:validate
node --check tests\performance\load-test.js
```

For the committed 500 VU profile, use the performance-only compose override so the API's normal `RATE_LIMIT_RPM=60` default is not edited. The k6 setup phase polls `/v1/events` for event IDs and fails fast if seed data is unavailable, because `GET /v1/events/{id}` coverage is mandatory in the mixed endpoint run.

```powershell
docker compose -f docker-compose.demo.yml -f tests/performance/docker-compose.performance.yml up -d --force-recreate
(Invoke-WebRequest -Uri http://127.0.0.1:8080/v1/events -UseBasicParsing).StatusCode
docker run --rm -e VIGILAFRICA_API_BASE_URL=http://host.docker.internal:8080 -v "${PWD}\tests\performance:/scripts:ro" grafana/k6 run /scripts/load-test.js
docker compose -f docker-compose.demo.yml -f tests/performance/docker-compose.performance.yml stop
```

The override sets `RATE_LIMIT_RPM=${PERF_RATE_LIMIT_RPM:-60000}` only for runs that include `tests/performance/docker-compose.performance.yml`. The base `docker-compose.demo.yml` and `.env.example` defaults remain unchanged.

## Verified Baseline Results

Final full staged k6 run from the initial 50 VU baseline:

| Metric | Result |
| --- | ---: |
| HTTP requests | 81 |
| Failed HTTP requests | 0.00% |
| Checks passed | 162 / 162 |
| p95 response time | 77.31 ms |
| Average response time | 19.22 ms |
| Median response time | 11 ms |
| Max response time | 212.55 ms |
| Iterations completed | 67 |
| Max virtual users | 50 |

Threshold outcome:

- `http_req_duration p(95)<200`: passed at 77.31 ms
- `http_req_failed rate<0.01`: passed at 0.00%

## Verified 500 VU Mixed-Endpoint Results

Final full staged k6 run with the committed weighted endpoint mix and the performance-only `RATE_LIMIT_RPM=60000` override:

| Metric | Result |
| --- | ---: |
| HTTP requests | 1,447 |
| Failed HTTP requests | 0.00% |
| Checks passed | 2,892 / 2,892 |
| p95 response time | 65.06 ms |
| Average response time | 18.38 ms |
| Median response time | 10.27 ms |
| Max response time | 443.73 ms |
| Iterations completed | 1,347 |
| Max virtual users | 500 |

Threshold outcome:

- `http_req_duration p(95)<200`: passed at 65.06 ms
- `http_req_failed rate<0.01`: passed at 0.00%

Checks covered:

- `GET /v1/events`
- `GET /v1/context`
- `GET /v1/states`
- `GET /v1/events/{id}`
- `GET /v1/enrichment-stats`
- `GET /health`

## Notes

- Native `k6` was not installed on PATH, so verification used the Docker runner.
- The first full run failed because `/v1/*` is protected by the API's default per-IP rate limit of `RATE_LIMIT_RPM=60`. The script was adjusted to use realistic dashboard think time while retaining 50 concurrent virtual users.
- The committed profile has since been raised to 500 VUs and expanded to the weighted endpoint mix above. Full 500 VU validation used the performance-only compose override so the test measured endpoint throughput rather than the per-IP rate limiter.
- The short smoke test also passed before the full run, with p95 at 21.55 ms and zero failed checks.
- The demo containers were stopped after verification.

## Outcome

The local developer-driven load test is ready for review. Against the demo API and database, the verified 500 VU mixed-endpoint run passed the 200 ms p95 performance budget with no HTTP failures.
