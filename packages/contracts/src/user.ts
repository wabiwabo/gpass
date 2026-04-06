export interface GarudaPassUser {
  id: string;
  nik_masked: string; // e.g., "************3456"
  name: string;
  email: string;
  phone: string;
  verified: boolean;
  auth_level: 0 | 1 | 2 | 3 | 4;
  created_at: string;
}
