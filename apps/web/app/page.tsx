import { Card } from "../components/Card";
import { PageHeader } from "../components/PageHeader";

const kpis = [
  {
    label: "Open Service Requests",
    value: "0",
    hint: "pending + assigned + in progress",
  },
  { label: "Active RFPs", value: "0", hint: "draft + published" },
  { label: "Equipment Assets", value: "0", hint: "registered devices" },
];

export default function DashboardPage() {
  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Overview of the current CMMS state."
      />

      <div className="kpi-grid">
        {kpis.map((kpi) => (
          <Card key={kpi.label}>
            <div className="kpi">
              <span className="kpi__label">{kpi.label}</span>
              <span className="kpi__value">{kpi.value}</span>
              <span className="kpi__hint">{kpi.hint}</span>
            </div>
          </Card>
        ))}
      </div>

      <Card title="Getting started">
        <p>
          Data is served by the Go API at{" "}
          <code>
            {process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}
          </code>
          . Set <code>NEXT_PUBLIC_TENANT_ID</code> and{" "}
          <code>NEXT_PUBLIC_USER_ID</code> in{" "}
          <code>apps/web/.env.local</code> to talk to the dev API. See{" "}
          <code>apps/web/.env.example</code>.
        </p>
      </Card>
    </>
  );
}
