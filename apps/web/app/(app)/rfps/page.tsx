import { Card } from "../../../components/Card";
import { PageHeader } from "../../../components/PageHeader";

export default function RfpsPage() {
  return (
    <>
      <PageHeader title="RFPs" description="Requests for proposal." />
      <Card title="Placeholder">
        <p>
          The API exposes single-RFP endpoints but no list endpoint. This page
          will list RFPs via <code>lib/api/rfps.ts</code> once{" "}
          <code>GET /api/v1/rfps</code> is available.
        </p>
      </Card>
    </>
  );
}
