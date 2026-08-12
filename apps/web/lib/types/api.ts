export type TenantStatus = "active" | "suspended" | "archived";

export interface Tenant {
  id: string;
  slug: string;
  name: string;
  status: TenantStatus;
  created_at: string;
  updated_at: string;
}

export interface CreateTenantInput {
  slug: string;
  name: string;
}

export type RequestPriority = "low" | "medium" | "high" | "critical";

export type RequestStatus =
  | "pending"
  | "assigned"
  | "in_progress"
  | "resolved"
  | "cancelled";

export interface ServiceRequest {
  id: string;
  tenant_id: string;
  equipment_id: string;
  title: string;
  description: string;
  priority: RequestPriority;
  status: RequestStatus;
  created_by: string;
  assigned_to: string | null;
  resolution_notes: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateServiceRequestInput {
  equipment_id: string;
  title: string;
  description: string;
  priority: RequestPriority;
}

export interface ServiceRequestList {
  requests: ServiceRequest[];
}

export interface TransitionServiceRequestInput {
  status: RequestStatus;
}

export interface RequestEvent {
  id: string;
  actor_id: string;
  from_status: RequestStatus;
  to_status: RequestStatus;
  created_at: string;
}

export interface RequestHistory {
  events: RequestEvent[];
}

export type EquipmentStatus = "operational" | "maintenance" | "retired";

export interface Equipment {
  id: string;
  asset_tag: string;
  name: string;
  serial_number: string;
  location: string;
  status: EquipmentStatus;
  created_at: string;
  updated_at: string;
}

export interface EquipmentList {
  equipment: Equipment[];
}

export type RfpStatus = "draft" | "published" | "closed" | "cancelled";

export interface Rfp {
  id: string;
  service_request_id: string;
  title: string;
  description: string;
  status: RfpStatus;
  due_at: string | null;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateRfpInput {
  service_request_id: string;
  title: string;
  description: string;
  due_at?: string | null;
}

export interface TransitionRfpInput {
  status: RfpStatus;
}

export interface Health {
  status: "ok";
}
