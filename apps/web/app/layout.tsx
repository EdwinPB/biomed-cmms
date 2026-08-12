import type { Metadata } from "next";
import "./globals.css";
import { Badge } from "../components/Badge";
import { Sidebar } from "../components/Sidebar";

export const metadata: Metadata = {
  title: "Biomed CMMS",
  description: "Biomedical asset and service management.",
};

const tenantId = process.env.NEXT_PUBLIC_TENANT_ID ?? "";

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const tenantLabel = tenantId
    ? `dev · ${tenantId.slice(0, 8)}…`
    : "dev · tenant not set";

  return (
    <html lang="en">
      <body>
        <div className="shell">
          <Sidebar />
          <div className="shell__body">
            <header className="header">
              <span className="header__brand">Biomed CMMS</span>
              <Badge tone="accent">{tenantLabel}</Badge>
            </header>
            <main className="main">{children}</main>
          </div>
        </div>
      </body>
    </html>
  );
}
