"use client";

import { useAuth } from "@/components/auth-provider";
import { loginUrl, logoutUrl } from "@/lib/api";

export default function Home() {
  const { authenticated, user } = useAuth();

  return (
    <main style={{ maxWidth: 600, margin: "0 auto", padding: 40 }}>
      <h1>GarudaPass</h1>
      <p>Indonesia&apos;s Unified Identity Platform</p>

      {authenticated && user ? (
        <div>
          <p>Welcome, <strong>{user.name || user.id}</strong></p>
          <form action={logoutUrl()} method="POST">
            <button type="submit">Logout</button>
          </form>
        </div>
      ) : (
        <div>
          <a href={loginUrl()}>
            <button>Login with GarudaPass</button>
          </a>
        </div>
      )}
    </main>
  );
}
