export interface IdentityProfile {
  user_id: string;
  name: string;
  nik_masked: string;
  verification_status: string;
  auth_level: number;
  created_at: string;
}

export interface Consent {
  id: string;
  requester_app: string;
  purpose: string;
  fields: Record<string, boolean>;
  status: string;
  granted_at: string;
  expires_at: string;
}

export interface CorporateEntity {
  id: string;
  name: string;
  entity_type: string;
  ahu_sk_number: string;
  status: string;
  role: string;
}

export interface SigningRequest {
  id: string;
  document_name: string;
  status: string;
  created_at: string;
  signed_document?: {
    pades_level: string;
    signature_timestamp: string;
  };
}
