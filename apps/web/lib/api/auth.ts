import { api } from "./client";
import type { AuthSession, LoginInput } from "../types/api";

export function login(input: LoginInput): Promise<AuthSession> {
  return api.post<AuthSession>("/api/v1/auth/login", input);
}

export function logout(): Promise<void> {
  return api.post<void>("/api/v1/auth/logout", {});
}

export function getMe(): Promise<AuthSession> {
  return api.get<AuthSession>("/api/v1/auth/me");
}
