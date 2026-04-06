import { Card, CardHeader, CardTitle, CardContent, Table, TableHeader, TableBody, TableRow, TableHead, TableCell, StatusBadge } from "@garudapass/ui";

const services = [
  { name: "BFF", port: 4000, description: "Backend for Frontend" },
  { name: "Identity", port: 4001, description: "Identity verification" },
  { name: "Dukcapil Sim", port: 4002, description: "Dukcapil simulator" },
  { name: "GarudaInfo", port: 4003, description: "Verified data API" },
  { name: "AHU Sim", port: 4004, description: "AHU/SABH simulator" },
  { name: "OSS Sim", port: 4005, description: "OSS/BKPM simulator" },
  { name: "GarudaCorp", port: 4006, description: "Corporate identity" },
  { name: "GarudaSign", port: 4007, description: "Digital signing" },
  { name: "Signing Sim", port: 4008, description: "Signing simulator" },
  { name: "GarudaPortal", port: 4009, description: "Developer portal" },
];

export default function SystemPage() {
  return (
    <div className="mx-auto max-w-7xl p-8 space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">System Health</h1>

      <Card>
        <CardHeader>
          <CardTitle>Service Status</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Service</TableHead>
                <TableHead>Port</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {services.map((svc) => (
                <TableRow key={svc.name}>
                  <TableCell className="font-medium">{svc.name}</TableCell>
                  <TableCell>{svc.port}</TableCell>
                  <TableCell className="text-gray-500">{svc.description}</TableCell>
                  <TableCell><StatusBadge status="ACTIVE" /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
