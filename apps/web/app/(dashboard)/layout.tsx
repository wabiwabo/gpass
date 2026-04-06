import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getSession } from "@/lib/api";
import Link from "next/link";

const navItems = [
  { href: "/dashboard", label: "Beranda" },
  { href: "/dashboard/identity", label: "Identitas" },
  { href: "/dashboard/consent", label: "Consent" },
  { href: "/dashboard/corporate", label: "Korporat" },
  { href: "/dashboard/signing", label: "Tanda Tangan" },
  { href: "/dashboard/settings", label: "Pengaturan" },
];

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore.toString();
  const session = await getSession(cookieHeader);

  if (!session) {
    redirect("/login");
  }

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="w-64 border-r bg-white">
        <div className="flex h-16 items-center border-b px-6">
          <Link href="/dashboard" className="text-lg font-bold text-brand-900">
            GarudaPass
          </Link>
        </div>
        <nav className="space-y-1 p-4">
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="block rounded-md px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="absolute bottom-0 w-64 border-t p-4">
          <p className="text-xs text-gray-500 truncate">{session.name}</p>
          <p className="text-xs text-gray-400 truncate">{session.email}</p>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 p-8">{children}</main>
    </div>
  );
}
