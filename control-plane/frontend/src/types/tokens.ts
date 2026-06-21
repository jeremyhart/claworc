export interface APIToken {
  id: number;
  name: string;
  prefix: string;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
}

export interface APITokenCreated extends APIToken {
  token: string;
}

export interface CreateAPITokenRequest {
  name: string;
  expires_in_days?: number;
}
