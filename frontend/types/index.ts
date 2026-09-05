export enum ProviderType {
  ORGANISATION = "ORGANISATION",
  INDIVIDUAL = "INDIVIDUAL",
}

export enum UserRole {
  PUBLIC = "PUBLIC",
  ADMIN = "ADMIN",
}

export interface User {
  id: string;
  phone: string;
  password_hash?: string;
  role: UserRole;
  full_name: string;
  created_at?: string;
}

export interface RegisterUserRequest {
  full_name: string;
  phone: string;
  password: string;
}

export interface LoginUserRequest {
  phone: string;
  password: string;
}

export interface Provider {
  id: string;
  type: ProviderType;
  name: string;
  authorized_person: string | null;
  govt_id: string;
  email: string;
  ph_no: string;
  state: string;
  dist: string;
  password_hash: string;
  registration_no?: string;
  cin?: string;
  website?: string;
  location?: string;
  last_updated_at?: string;
  created_at?: string;
}

export interface LoginProviderRequest {
  email?: string;
  ph_no?: string;
  password: string;
}

export interface RegisterProviderRequest {
  type: ProviderType;
  name: string;
  authorized_person?: string;
  registration_no?: string;
  govt_id: string;
  cin?: string;
  email: string;
  ph_no: string;
  website?: string;
  state: string;
  dist: string;
  location: string;
  password: string;
}

export interface AuthResponse<TProfile> {
  token: string;
  account_type: "user" | "provider";
  profile: TProfile;
}