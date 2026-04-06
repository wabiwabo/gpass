// GarudaPass Smoke Test — k6
// Validates all service health endpoints respond within SLA.
// Run: k6 run tests/load/smoke.js

import http from "k6/http";
import { check, group } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://localhost";

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    http_req_duration: ["p(99)<500"], // 99th percentile under 500ms
    http_req_failed: ["rate<0.01"],   // <1% failure rate
  },
};

const services = [
  { name: "BFF", url: `${BASE_URL}:4000/health` },
  { name: "Identity", url: `${BASE_URL}:4001/health` },
  { name: "Dukcapil Sim", url: `${BASE_URL}:4002/health` },
  { name: "GarudaInfo", url: `${BASE_URL}:4003/health` },
  { name: "AHU Sim", url: `${BASE_URL}:4004/health` },
  { name: "OSS Sim", url: `${BASE_URL}:4005/health` },
  { name: "GarudaCorp", url: `${BASE_URL}:4006/health` },
  { name: "GarudaSign", url: `${BASE_URL}:4007/health` },
  { name: "Signing Sim", url: `${BASE_URL}:4008/health` },
  { name: "GarudaPortal", url: `${BASE_URL}:4009/health` },
];

export default function () {
  for (const svc of services) {
    group(svc.name, () => {
      const res = http.get(svc.url, { timeout: "5s" });
      check(res, {
        [`${svc.name} status 200`]: (r) => r.status === 200,
        [`${svc.name} has status ok`]: (r) => {
          try {
            return JSON.parse(r.body).status === "ok";
          } catch {
            return false;
          }
        },
        [`${svc.name} < 200ms`]: (r) => r.timings.duration < 200,
      });
    });
  }
}
