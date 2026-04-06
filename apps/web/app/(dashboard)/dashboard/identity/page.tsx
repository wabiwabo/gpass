import { Card, CardHeader, CardTitle, CardContent, Badge } from "@garudapass/ui";

export default function IdentityPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Profil Identitas</h1>

      <Card>
        <CardHeader>
          <CardTitle>Data Kependudukan</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 sm:grid-cols-2">
            <div>
              <dt className="text-sm font-medium text-gray-500">NIK</dt>
              <dd className="mt-1 text-sm text-gray-900">****-****-****-1234</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Status</dt>
              <dd className="mt-1"><Badge variant="success">Terverifikasi</Badge></dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Level Autentikasi</dt>
              <dd className="mt-1 text-sm text-gray-900">L2 (OTP + Passkey)</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Sumber</dt>
              <dd className="mt-1 text-sm text-gray-900">DJK (Dukcapil)</dd>
            </div>
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}
