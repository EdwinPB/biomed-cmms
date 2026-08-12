import { api } from "./client";
import {
  CreateRfpInput,
  Rfp,
  TransitionRfpInput,
} from "../types/api";

export function createRfp(input: CreateRfpInput): Promise<Rfp> {
  return api.post<Rfp>("/api/v1/rfps", input);
}

export function transitionRfpStatus(
  id: string,
  input: TransitionRfpInput,
): Promise<Rfp> {
  return api.patch<Rfp>(`/api/v1/rfps/${id}/status`, input);
}

export function getRfp(id: string): Promise<Rfp> {
  return api.get<Rfp>(`/api/v1/rfps/${id}`);
}

export function getRfpByServiceRequest(serviceRequestId: string): Promise<Rfp> {
  return api.get<Rfp>(`/api/v1/service-requests/${serviceRequestId}/rfp`);
}
