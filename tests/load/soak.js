// GarudaPass Soak Test — k6
// Runs steady load for extended period to detect memory leaks and degradation.
// Run: k6 run --duration 30m tests/load/soak.js

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost";
const errorRate = new Rate("errors");

export const options = {
  stages: [
    { duration: "2m", target: 50 },   // ramp up
    { duration: "26m", target: 50 },  // steady state
    { duration: "2m", target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    http_req_failed: ["rate<0.01"], // stricter for soak: <1%
    errors: ["rate<0.01"],
  },
};

const healthEndpoints = [4000, 4001, 4003, 4006, 4007, 4009];

export default function () {
  const port = healthEndpoints[Math.floor(Math.random() * healthEndpoints.length)];
  const res = http.get(`${BASE_URL}:${port}/health`, {
    timeout: "5s",
    headers: { "X-Request-Id": `soak-${__VU}-${__ITER}` },
  });

  const ok = check(res, {
    "status 200": (r) => r.status === 200,
    "latency < 500ms": (r) => r.timings.duration < 500,
  });

  errorRate.add(!ok);
  sleep(0.5 + Math.random()); // 0.5-1.5s think time
}
