"use client";
import { Button } from "@garudapass/ui";

export default function LoginPage() {
  const handleLogin = () => {
    window.location.href = `${process.env.NEXT_PUBLIC_BFF_URL || "http://localhost:4000"}/auth/login`;
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50">
      <div className="w-full max-w-md space-y-8 rounded-lg border bg-white p-8 shadow-sm">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-brand-900">GarudaPass</h1>
          <p className="mt-2 text-sm text-gray-600">
            Masuk ke akun GarudaPass Anda
          </p>
        </div>

        <Button className="w-full" size="lg" onClick={handleLogin}>
          Masuk dengan GarudaPass
        </Button>

        <p className="text-center text-sm text-gray-500">
          Belum punya akun?{" "}
          <a href="/register" className="font-medium text-brand-600 hover:text-brand-700">
            Daftar
          </a>
        </p>
      </div>
    </div>
  );
}
