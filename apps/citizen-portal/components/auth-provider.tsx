"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import type { SessionInfo } from "@gpass/contracts";
import { getSession } from "@/lib/api";

const AuthContext = createContext<SessionInfo>({
  user: null,
  authenticated: false,
  csrf_token: "",
});

export function useAuth() {
  return useContext(AuthContext);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionInfo>({
    user: null,
    authenticated: false,
    csrf_token: "",
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getSession()
      .then(setSession)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div style={{ padding: 40, textAlign: "center" }}>Loading...</div>;
  }

  return <AuthContext.Provider value={session}>{children}</AuthContext.Provider>;
}
