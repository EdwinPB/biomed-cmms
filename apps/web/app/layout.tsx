import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Biomed CMMS",
  description: "Biomedical asset and service management.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="es">
      <body>{children}</body>
    </html>
  );
}
