import { AuthProvider } from "../../components/AuthProvider";
import { HeaderUser } from "../../components/HeaderUser";
import { Sidebar } from "../../components/Sidebar";

export default function AppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthProvider>
      <div className="shell">
        <Sidebar />
        <div className="shell__body">
          <header className="header">
            <span className="header__brand">Biomed CMMS</span>
            <HeaderUser />
          </header>
          <main className="main">{children}</main>
        </div>
      </div>
    </AuthProvider>
  );
}
