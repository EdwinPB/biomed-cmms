import { ApiError, getMe } from "./api";
import type { AuthSession } from "./types/api";

export async function getSession(): Promise<AuthSession | null> {
  try {
    return await getMe();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      return null;
    }
    throw err;
  }
}
