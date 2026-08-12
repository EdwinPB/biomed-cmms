import { api } from "./client";
import {
  CreateServiceRequestInput,
  RequestHistory,
  ServiceRequest,
  TransitionServiceRequestInput,
} from "../types/api";

export function createServiceRequest(
  input: CreateServiceRequestInput,
): Promise<ServiceRequest> {
  return api.post<ServiceRequest>("/api/v1/requests", input);
}

export function transitionServiceRequest(
  id: string,
  input: TransitionServiceRequestInput,
): Promise<ServiceRequest> {
  return api.patch<ServiceRequest>(`/api/v1/requests/${id}/status`, input);
}

export function getRequestHistory(id: string): Promise<RequestHistory> {
  return api.get<RequestHistory>(`/api/v1/requests/${id}/history`);
}
