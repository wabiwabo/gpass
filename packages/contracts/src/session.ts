export interface SessionInfo {
  user: {
    id: string;
    name: string;
    email: string;
    verified: boolean;
    auth_level: number;
  } | null;
  authenticated: boolean;
  csrf_token: string;
}
