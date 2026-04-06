"use client";
import { useState } from "react";
import { Button, Input, Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@garudapass/ui";

export default function RegisterPage() {
  const [nik, setNik] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (nik.length !== 16) {
      setError("NIK harus 16 digit");
      return;
    }

    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_BFF_URL || "http://localhost:4000"}/api/v1/identity/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ nik, name }),
      });

      if (!res.ok) {
        const data = await res.json();
        setError(data.error || "Registrasi gagal");
        return;
      }

      window.location.href = "/dashboard";
    } catch {
      setError("Terjadi kesalahan. Coba lagi.");
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Daftar GarudaPass</CardTitle>
          <CardDescription>Verifikasi identitas Anda dengan NIK</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            <Input
              label="Nomor Induk Kependudukan (NIK)"
              placeholder="16 digit NIK"
              value={nik}
              onChange={(e) => setNik(e.target.value.replace(/\D/g, "").slice(0, 16))}
              maxLength={16}
              error={error && nik.length !== 16 ? "NIK harus 16 digit" : undefined}
            />
            <Input
              label="Nama Lengkap"
              placeholder="Sesuai KTP"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            {error && <p className="text-sm text-red-600">{error}</p>}
          </CardContent>
          <CardFooter>
            <Button type="submit" className="w-full">
              Verifikasi & Daftar
            </Button>
          </CardFooter>
        </form>
      </Card>
    </div>
  );
}
