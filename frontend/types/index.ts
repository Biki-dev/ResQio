/**
 * Shared domain types for ResQio.
 *
 * These interfaces intentionally mirror the backend PostgreSQL/PostGIS
 * schema field-for-field so that request/response payloads can be typed
 * directly against them without a translation layer. Keep this file in
 * sync with the backend migrations.
 */

// ---- Enums --------------------------------------------------------------

/** Matches DB enum `provider_type` */
export enum ProviderType {
  ORGANISATION = "ORGANISATION",
  INDIVIDUAL = "INDIVIDUAL",
}

/** Matches DB enum `user_role` (default 'PUBLIC') */
export enum UserRole {
  PUBLIC = "PUBLIC",
  ADMIN = "ADMIN",
}

// ---- Table: users ---------------------------------------------------------

/** Matches DB table `users` */
export interface User {
  id: string; // uuid, pk
  phone: string; // varchar(20), unique, not null
  password_hash: string; // text, not null — never sent to/from the client
  role: UserRole; // user_role, default 'PUBLIC'
  full_name: string; // varchar(100), not null
}

/** Payload for POST /api/auth/register/user — public requester sign-up */
export interface RegisterUserRequest {
  full_name: string;
  phone: string;
  password: string; // hashed server-side into password_hash
}

// ---- Table: providers ------------------------------------------------------

/** Matches DB table `providers` */
export interface Provider {
  id: string; // uuid, pk
  type: ProviderType; // provider_type, default 'ORGANISATION'
  name: string; // varchar(255), not null
  authorized_person: string | null; // varchar(255), nullable
  govt_id: string; // varchar(50), not null
  email: string; // varchar(255), not null
  ph_no: string; // varchar(20), not null
  state: string; // varchar(255), not null
  dist: string; // varchar(255), not null
  password_hash: string; // text, not null — never sent to/from the client
}

/** Payload for POST /api/auth/register/provider — org / solo provider sign-up */
export interface RegisterProviderRequest {
  type: ProviderType;
  name: string;
  authorized_person?: string;
  govt_id: string;
  email: string;
  ph_no: string;
  state: string;
  dist: string;
  password: string; // hashed server-side into password_hash
}

// ---- Generic API envelope --------------------------------------------------

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}
