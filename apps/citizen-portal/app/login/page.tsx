"use client";

import { useEffect } from "react";
import { loginUrl } from "@/lib/api";

export default function LoginPage() {
  useEffect(() => {
    window.location.href = loginUrl();
  }, []);

  return (
    <main style={{ padding: 40, textAlign: "center" }}>
      <p>Redirecting to GarudaPass login...</p>
    </main>
  );
}
