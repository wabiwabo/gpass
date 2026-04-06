// GarudaPass Stress Test — k6
// Simulates increasing load to find breaking points.
// Run: k6 run tests/load/stress.js

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost";

const errorRate = new Rate("errors");
const apiLatency = new Trend("api_latency", true);

export const options = {
  stages: [
    { duration: "1m", target: 10 },   // ramp up to 10 VUs
    { duration: "3m", target: 50 },   // ramp up to 50 VUs
    { duration: "5m", target: 100 },  // ramp up to 100 VUs
    { duration: "3m", target: 200 },  // peak: 200 VUs
    { duration: "2m", target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ["p(95)<1000", "p(99)<2000"], // SLO: P95 < 1s, P99 < 2s
    http_req_failed: ["rate<0.05"],                   // <5% failure rate
    errors: ["rate<0.05"],
  },
};

const endpoints = [
  { method: "GET", path: "/health", port: 4000, weight: 0.3 },
  { method: "GET", path: "/health", port: 4001, weight: 0.2 },
  { method: "GET", path: "/health", port: 4006, weight: 0.2 },
  { method: "GET", path: "/health", port: 4007, weight: 0.15 },
  { method: "GET", path: "/health", port: 4009, weight: 0.15 },
];

function selectEndpoint() {
  const rand = Math.random();
  let cumulative = 0;
  for (const ep of endpoints) {
    cumulative += ep.weight;
    if (rand <= cumulative) return ep;
  }
  return endpoints[0];
}

export default function () {
  const ep = selectEndpoint();
  const url = `${BASE_URL}:${ep.port}${ep.path}`;

  const res = http.request(ep.method, url, null, {
    timeout: "10s",
    headers: {
      "X-Request-Id": `k6-${__VU}-${__ITER}`,
    },
  });

  apiLatency.add(res.timings.duration);

  const success = check(res, {
    "status is 200": (r) => r.status === 200,
    "response time < 2s": (r) => r.timings.duration < 2000,
  });

  errorRate.add(!success);
  sleep(0.1 + Math.random() * 0.4); // 100-500ms think time
}

export function handleSummary(data) {
  return {
    stdout: JSON.stringify(
      {
        timestamp: new Date().toISOString(),
        total_requests: data.metrics.http_reqs.values.count,
        avg_duration_ms: data.metrics.http_req_duration.values.avg.toFixed(2),
        p95_duration_ms: data.metrics.http_req_duration.values["p(95)"].toFixed(2),
        p99_duration_ms: data.metrics.http_req_duration.values["p(99)"].toFixed(2),
        error_rate: (data.metrics.http_req_failed.values.rate * 100).toFixed(2) + "%",
        max_vus: data.metrics.vus_max.values.max,
      },
      null,
      2
    ),
  };
}
