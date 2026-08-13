"use client";

import { useState } from "react";
import { useAuth } from "./AuthProvider";
import { Button } from "./Button";

export function HeaderUser() {
  const { user, tenant, status, logout } = useAuth();
  const [loggingOut, setLoggingOut] = useState(false);

  if (status === "loading" || !user) {
    return null;
  }

  async function handleLogout() {
    setLoggingOut(true);
    await logout();
  }

  return (
    <div className="header-user">
      <div className="header-user__meta">
        <span className="header-user__name">{user.full_name || user.email}</span>
        {tenant ? (
          <span className="header-user__tenant">{tenant.name}</span>
        ) : null}
      </div>
      <Button variant="secondary" onClick={handleLogout} disabled={loggingOut}>
        {loggingOut ? "Cerrando sesión…" : "Cerrar sesión"}
      </Button>
    </div>
  );
}
