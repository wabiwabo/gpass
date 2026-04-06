import type { SessionInfo } from "@gpass/contracts";

const BFF_BASE = "";

export async function getSession(): Promise<SessionInfo> {
  const res = await fetch(`${BFF_BASE}/auth/session`, {
    credentials: "include",
  });
  if (!res.ok) {
    return { user: null, authenticated: false, csrf_token: "" };
  }
  return res.json();
}

export function loginUrl(): string {
  return `/auth/login`;
}

export function logoutUrl(): string {
  return `/auth/logout`;
}
