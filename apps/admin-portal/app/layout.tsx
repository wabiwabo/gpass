export const metadata = {
  title: "GarudaPass Admin",
  description: "GarudaPass Administration Portal",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>{children}</body>
    </html>
  );
}
