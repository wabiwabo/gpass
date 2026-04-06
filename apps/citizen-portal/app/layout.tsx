import { AuthProvider } from "@/components/auth-provider";

export const metadata = {
  title: "GarudaPass",
  description: "Indonesia's Unified Identity Platform",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
