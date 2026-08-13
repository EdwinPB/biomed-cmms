"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { usePathname, useRouter } from "next/navigation";
import { logout as apiLogout } from "../lib/api";
import { getSession } from "../lib/auth";
import type { AuthSession } from "../lib/types/api";

type AuthStatus = "loading" | "ready";

type AuthContextValue = {
  status: AuthStatus;
  session: AuthSession | null;
  user: AuthSession["user"] | null;
  tenant: AuthSession["tenant"] | null;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [session, setSession] = useState<AuthSession | null>(null);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        const current = await getSession();
        if (cancelled) return;
        if (current) {
          setSession(current);
          setStatus("ready");
        } else {
          router.replace("/login");
        }
      } catch {
        if (!cancelled) {
          router.replace("/login");
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [router, pathname]);

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } catch {
      // Session state is cleared locally regardless of network outcome.
    }
    setSession(null);
    router.replace("/login");
  }, [router]);

  if (status === "loading") {
    return (
      <div className="shell">
        <div className="shell__body">
          <main className="main">
            <p className="loading">Cargando…</p>
          </main>
        </div>
      </div>
    );
  }

  if (!session) {
    return null;
  }

  return (
    <AuthContext.Provider
      value={{
        status,
        session,
        user: session.user,
        tenant: session.tenant,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
