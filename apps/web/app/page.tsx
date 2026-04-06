import { Button } from "@garudapass/ui";
import Link from "next/link";

export default function LandingPage() {
  return (
    <div className="flex min-h-screen flex-col">
      {/* Header */}
      <header className="border-b bg-white">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4">
          <h1 className="text-xl font-bold text-brand-900">GarudaPass</h1>
          <div className="flex gap-3">
            <Link href="/login">
              <Button variant="outline" size="sm">Masuk</Button>
            </Link>
            <Link href="/register">
              <Button size="sm">Daftar</Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <main className="flex flex-1 flex-col items-center justify-center px-4">
        <div className="max-w-2xl text-center">
          <h2 className="text-4xl font-bold tracking-tight text-gray-900 sm:text-5xl">
            Infrastruktur Identitas Digital Indonesia
          </h2>
          <p className="mt-6 text-lg text-gray-600">
            Verifikasi identitas, otorisasi korporat, tanda tangan digital, dan
            berbagi data terverifikasi dalam satu platform.
          </p>
          <div className="mt-10 flex justify-center gap-4">
            <Link href="/register">
              <Button size="lg">Mulai Gratis</Button>
            </Link>
            <Link href="https://docs.garudapass.id" target="_blank">
              <Button variant="outline" size="lg">Dokumentasi API</Button>
            </Link>
          </div>
        </div>

        {/* Features */}
        <div className="mx-auto mt-20 grid max-w-5xl gap-8 sm:grid-cols-2 lg:grid-cols-4">
          {[
            { title: "GarudaPass Login", desc: "FAPI 2.0 OIDC dengan verifikasi NIK" },
            { title: "GarudaInfo", desc: "Data pribadi terverifikasi berbasis consent" },
            { title: "GarudaCorp", desc: "Identitas korporat dari AHU & OSS" },
            { title: "GarudaSign", desc: "Tanda tangan digital PAdES-B-LTA" },
          ].map((f) => (
            <div key={f.title} className="rounded-lg border bg-white p-6 shadow-sm">
              <h3 className="font-semibold text-gray-900">{f.title}</h3>
              <p className="mt-2 text-sm text-gray-500">{f.desc}</p>
            </div>
          ))}
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t bg-white py-8">
        <div className="mx-auto max-w-7xl px-4 text-center text-sm text-gray-500">
          &copy; 2026 GarudaPass. Seluruh hak dilindungi.
        </div>
      </footer>
    </div>
  );
}
