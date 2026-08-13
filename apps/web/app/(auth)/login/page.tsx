"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError, login } from "../../../lib/api";
import { getSession } from "../../../lib/auth";
import { Button } from "../../../components/Button";
import { Card } from "../../../components/Card";

export default function LoginPage() {
  const router = useRouter();
  const [tenantSlug, setTenantSlug] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    let cancelled = false;

    getSession()
      .then((session) => {
        if (cancelled) return;
        if (session) {
          router.replace("/requests");
        } else {
          setChecking(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setChecking(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [router]);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      await login({
        tenant_slug: tenantSlug.trim(),
        email: email.trim(),
        password,
      });
      router.replace("/requests");
    } catch (err) {
      if (err instanceof ApiError) {
        setError("Credenciales inválidas");
      } else {
        setError(
          "No se pudo conectar con el servidor. Inténtalo de nuevo.",
        );
      }
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-card">
      <Card>
        <h1 className="auth-card__title">Iniciar sesión</h1>
        <p className="auth-card__sub">
          Accede a tu organización en Biomed CMMS.
        </p>

        <form className="form" onSubmit={handleSubmit} noValidate>
          {error ? <p className="alert alert--error">{error}</p> : null}

          <div className="field">
            <label className="field__label" htmlFor="tenant-slug">
              Organización
            </label>
            <input
              id="tenant-slug"
              className="field__input"
              type="text"
              autoComplete="organization"
              placeholder="local-dev"
              value={tenantSlug}
              onChange={(e) => setTenantSlug(e.target.value)}
              disabled={submitting}
              autoFocus
            />
          </div>

          <div className="field">
            <label className="field__label" htmlFor="email">
              Correo electrónico
            </label>
            <input
              id="email"
              className="field__input"
              type="email"
              autoComplete="email"
              placeholder="dev@local.test"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={submitting}
            />
          </div>

          <div className="field">
            <label className="field__label" htmlFor="password">
              Contraseña
            </label>
            <input
              id="password"
              className="field__input"
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={submitting}
            />
          </div>

          <div className="form__actions">
            <Button type="submit" disabled={submitting || checking}>
              {submitting ? "Iniciando sesión…" : "Iniciar sesión"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
