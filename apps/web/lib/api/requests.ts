import { api } from "./client";
import {
  CreateServiceRequestInput,
  RequestHistory,
  ServiceRequest,
  ServiceRequestList,
  TransitionServiceRequestInput,
} from "../types/api";

export function listServiceRequests(): Promise<ServiceRequestList> {
  return api.get<ServiceRequestList>("/api/v1/requests");
}

export function getServiceRequest(id: string): Promise<ServiceRequest> {
  return api.get<ServiceRequest>(`/api/v1/requests/${id}`);
}

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
