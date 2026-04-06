const BFF_URL = process.env.BFF_URL || "http://localhost:4000";

export class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = "APIError";
  }
}

export async function bffFetch<T>(
  path: string,
  options?: RequestInit & { cookies?: string }
): Promise<T> {
  const url = `${BFF_URL}${path}`;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options?.headers as Record<string, string>),
  };

  if (options?.cookies) {
    headers["Cookie"] = options.cookies;
  }

  const res = await fetch(url, {
    ...options,
    headers,
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.text();
    throw new APIError(res.status, body);
  }

  return res.json();
}

// Session
export interface Session {
  user_id: string;
  email: string;
  name: string;
  auth_level: number;
}

export async function getSession(cookies: string): Promise<Session | null> {
  try {
    return await bffFetch<Session>("/api/session", { cookies });
  } catch {
    return null;
  }
}
