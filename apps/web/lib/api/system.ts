import { api } from "./client";
import {
  CreateTenantInput,
  Health,
  Tenant,
} from "../types/api";

export function createTenant(input: CreateTenantInput): Promise<Tenant> {
  return api.post<Tenant>("/api/v1/tenants", input);
}

export function getHealth(): Promise<Health> {
  return api.get<Health>("/health");
}
