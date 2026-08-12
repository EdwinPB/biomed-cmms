import type { ReactNode } from "react";

type BadgeProps = {
  tone?: "neutral" | "accent" | "ok" | "warn" | "bad";
  children: ReactNode;
};

export function Badge({ tone = "neutral", children }: BadgeProps) {
  return <span className={`badge badge--${tone}`}>{children}</span>;
}
