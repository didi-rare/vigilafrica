import http from "k6/http";
import { check, sleep } from "k6";

const API_BASE_URL = (__ENV.VIGILAFRICA_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const TARGET_VUS = Number(__ENV.VIGILAFRICA_TARGET_VUS || "500");
const RAMP_UP_DURATION = __ENV.VIGILAFRICA_RAMP_UP_DURATION || "30s";
const HOLD_DURATION = __ENV.VIGILAFRICA_HOLD_DURATION || "1m";
const RAMP_DOWN_DURATION = __ENV.VIGILAFRICA_RAMP_DOWN_DURATION || "30s";
const THINK_TIME_SECONDS = Number(__ENV.VIGILAFRICA_THINK_TIME_SECONDS || "30");
const SETUP_WAIT_SECONDS = Number(__ENV.VIGILAFRICA_SETUP_WAIT_SECONDS || "60");
const SETUP_POLL_SECONDS = Number(__ENV.VIGILAFRICA_SETUP_POLL_SECONDS || "2");

export const options = {
  stages: [
    { duration: RAMP_UP_DURATION, target: TARGET_VUS },
    { duration: HOLD_DURATION, target: TARGET_VUS },
    { duration: RAMP_DOWN_DURATION, target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<200"],
    http_req_failed: ["rate<0.01"],
  },
};

const eventListRequests = [
  { name: "all-events", params: { limit: "50" }, weight: 4 },
  { name: "nigeria-events", params: { country: "Nigeria", limit: "50" }, weight: 2 },
  { name: "ghana-events", params: { country: "Ghana", limit: "50" }, weight: 2 },
  { name: "flood-events", params: { category: "floods", limit: "50" }, weight: 1 },
  { name: "wildfire-events", params: { category: "wildfires", limit: "50" }, weight: 1 },
  { name: "open-nigeria-floods", params: { country: "Nigeria", category: "floods", status: "open", limit: "25" }, weight: 1 },
  { name: "paged-events", params: { limit: "25", offset: "25" }, weight: 1 },
];

const stateRequests = [
  { name: "states-all", params: {}, weight: 1 },
  { name: "states-nigeria", params: { country: "Nigeria" }, weight: 2 },
  { name: "states-ghana", params: { country: "Ghana" }, weight: 2 },
];

const endpointMix = [
  { endpoint: "events-list", weight: 60 },
  { endpoint: "context", weight: 15 },
  { endpoint: "states", weight: 10 },
  { endpoint: "event-detail", weight: 10 },
  { endpoint: "enrichment-stats", weight: 3 },
  { endpoint: "health", weight: 2 },
];

const weightedEndpoints = endpointMix.flatMap((request) => Array(request.weight).fill(request.endpoint));
const weightedEventListRequests = eventListRequests.flatMap((request) => Array(request.weight).fill(request));
const weightedStateRequests = stateRequests.flatMap((request) => Array(request.weight).fill(request));

function buildURL(path, params) {
  const query = Object.entries(params)
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
    .join("&");

  return query ? `${API_BASE_URL}${path}?${query}` : `${API_BASE_URL}${path}`;
}

function randomItem(items) {
  return items[Math.floor(Math.random() * items.length)];
}

function safeJSON(response) {
  try {
    return response.json();
  } catch (_) {
    return null;
  }
}

export function setup() {
  const deadline = Date.now() + SETUP_WAIT_SECONDS * 1000;
  let lastStatus = 0;

  while (Date.now() <= deadline) {
    const response = http.get(buildURL("/v1/events", { limit: "50" }), {
      tags: { endpoint: "/v1/events", scenario: "setup-event-ids" },
    });
    lastStatus = response.status;

    const body = safeJSON(response);
    const ids = Array.isArray(body?.data) ? body.data.map((event) => event.id).filter(Boolean) : [];
    if (ids.length > 0) {
      return { eventIds: ids };
    }

    sleep(SETUP_POLL_SECONDS);
  }

  throw new Error(
    `No event IDs discovered from /v1/events after ${SETUP_WAIT_SECONDS}s (last status: ${lastStatus}). ` +
      "Ensure demo seed data has finished loading before running the mixed-endpoint load test.",
  );
}

function getEventsList() {
  const request = randomItem(weightedEventListRequests);
  const response = http.get(buildURL("/v1/events", request.params), {
    tags: { endpoint: "/v1/events", scenario: request.name },
  });

  check(response, {
    "status is 200": (res) => res.status === 200,
    "response includes data array": (res) => {
      const body = safeJSON(res);
      return Array.isArray(body?.data);
    },
  });
}

function getEventDetail(eventIds) {
  if (eventIds.length === 0) {
    throw new Error("event-detail scenario requires setup-discovered event IDs");
  }

  const id = randomItem(eventIds);
  const response = http.get(`${API_BASE_URL}/v1/events/${id}`, {
    tags: { endpoint: "/v1/events/{id}", scenario: "event-detail" },
  });

  check(response, {
    "detail status is 200": (res) => res.status === 200,
    "detail response includes id": (res) => safeJSON(res)?.id === id,
  });
}

function getContext() {
  const response = http.get(`${API_BASE_URL}/v1/context`, {
    headers: { "X-Forwarded-For": "102.89.46.1" },
    tags: { endpoint: "/v1/context", scenario: "geo-context" },
  });

  check(response, {
    "context status is 200": (res) => res.status === 200,
    "context response includes nearby_events": (res) => Array.isArray(safeJSON(res)?.nearby_events),
  });
}

function getStates() {
  const request = randomItem(weightedStateRequests);
  const response = http.get(buildURL("/v1/states", request.params), {
    tags: { endpoint: "/v1/states", scenario: request.name },
  });

  check(response, {
    "states status is 200": (res) => res.status === 200,
    "states response includes states array": (res) => Array.isArray(safeJSON(res)?.states),
  });
}

function getEnrichmentStats() {
  const response = http.get(`${API_BASE_URL}/v1/enrichment-stats`, {
    tags: { endpoint: "/v1/enrichment-stats", scenario: "enrichment-stats" },
  });

  check(response, {
    "enrichment stats status is 200": (res) => res.status === 200,
    "enrichment stats response includes stats array": (res) => Array.isArray(safeJSON(res)?.stats),
  });
}

function getHealth() {
  const response = http.get(`${API_BASE_URL}/health`, {
    tags: { endpoint: "/health", scenario: "health" },
  });

  check(response, {
    "health status is 200": (res) => res.status === 200,
    "health response includes status": (res) => typeof safeJSON(res)?.status === "string",
  });
}

export default function (data) {
  const endpoint = randomItem(weightedEndpoints);

  if (endpoint === "events-list") {
    getEventsList();
  } else if (endpoint === "context") {
    getContext();
  } else if (endpoint === "states") {
    getStates();
  } else if (endpoint === "event-detail") {
    getEventDetail(data.eventIds);
  } else if (endpoint === "enrichment-stats") {
    getEnrichmentStats();
  } else {
    getHealth();
  }

  // Simulate dashboard-like users instead of a tight request loop. For local
  // 500 VU runs, raise RATE_LIMIT_RPM or this will intentionally surface 429s.
  sleep(THINK_TIME_SECONDS + Math.random() * 15);
}
