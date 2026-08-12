import type { ReactNode } from "react";

type CardProps = {
  title?: string;
  className?: string;
  children: ReactNode;
};

export function Card({ title, className = "", children }: CardProps) {
  return (
    <section className={`card ${className}`.trim()}>
      {title ? <h3 className="card__title">{title}</h3> : null}
      {children}
    </section>
  );
}
