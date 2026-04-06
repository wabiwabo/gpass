import { Card, CardHeader, CardTitle, CardContent, CardFooter, Input, Button } from "@garudapass/ui";

export default function SettingsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Pengaturan Akun</h1>

      <Card>
        <CardHeader>
          <CardTitle>Profil</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input label="Email" type="email" disabled placeholder="user@example.com" />
          <Input label="Nama" placeholder="Nama lengkap" />
        </CardContent>
        <CardFooter>
          <Button>Simpan Perubahan</Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Keamanan</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium text-gray-900">Two-Factor Authentication</p>
              <p className="text-sm text-gray-500">Tambahkan lapisan keamanan ekstra</p>
            </div>
            <Button variant="outline" size="sm">Aktifkan</Button>
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium text-gray-900">Passkey</p>
              <p className="text-sm text-gray-500">Login tanpa password</p>
            </div>
            <Button variant="outline" size="sm">Daftarkan</Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
