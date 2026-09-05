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

export enum ResourceCategory {
  FOOD = "FOOD",
  WATER = "WATER",
  MEDICINE = "MEDICINE",
  SHELTER = "SHELTER",
  EQUIPMENT = "EQUIPMENT",
  VOLUNTEER = "VOLUNTEER",
  OTHER = "OTHER",
}

export enum RequestPriority {
  LOW = "LOW",
  MEDIUM = "MEDIUM",
  HIGH = "HIGH",
  CRITICAL = "CRITICAL",
}

export enum RequestStatus {
  SUBMITTED = "SUBMITTED",
  ACKNOWLEDGED = "ACKNOWLEDGED",
  IN_PROGRESS = "IN_PROGRESS",
  FULFILLED = "FULFILLED",
  CANCELLED = "CANCELLED",
}

export interface SubmitRoadHazardRequest {
  name: string;
  phone_number: string;
  hazard_type?: string;
  severity?: string;
  description?: string;
  photo_url?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
}

export interface RoadHazardResponse {
  id: string;
  reporter_id?: string;
  name: string;
  phone_number: string;
  hazard_type: string;
  severity: string;
  location: string;
  description: string;
  is_verified: boolean;
  created_at: string;
}

export interface SubmitAssistanceRequest {
  name: string;
  identity?: string;
  phone_number: string;
  things_needed: string;
  category?: ResourceCategory | string;
  amount: number;
  description?: string;
  photo_url?: string;
  priority?: RequestPriority | string;
  location?: string;
  latitude?: number;
  longitude?: number;
  address_text?: string;
}

export interface AssistanceRequestResponse {
  id: string;
  requester_id?: string;
  tracking_code: string;
  category: ResourceCategory | string;
  quantity_needed: number;
  description: string;
  priority: RequestPriority | string;
  status: RequestStatus | string;
  assigned_coordinator_id?: string;
  location: string;
  address_text: string;
  requester_name: string;
  contact_phone: string;
  created_at: string;
  updated_at: string;
}

export interface SubmitDisasterReportRequest {
  reporter_id?: string;
  embedding?: number[];
  vector_str?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
}

export interface DisasterReportResponse {
  id: string;
  reporter_id: string;
  location: string;
  created_at: string;
}

export interface MutualAidItemRequest {
  item_name: string;
  quantity: number;
  description?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
}

export interface MutualAidItemResponse {
  id: string;
  user_id: string;
  item_name: string;
  quantity: number;
  description: string;
  location: string;
  is_claimed: boolean;
  claimed_by_user_id?: string;
  created_at: string;
}

export interface ResourceRequest {
  title: string;
  description?: string;
  category: ResourceCategory | string;
  total_capacity: number;
  current_capacity: number;
  unit?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
  contact_phone?: string;
}

export interface ResourceResponse extends ResourceRequest {
  id: string;
  provider_id: string;
  description: string;
  category: ResourceCategory | string;
  unit: string;
  status: string;
  location: string;
  contact_phone: string;
  last_updated_at: string;
  created_at: string;
}