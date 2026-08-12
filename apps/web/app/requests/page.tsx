import { Card } from "../../components/Card";
import { PageHeader } from "../../components/PageHeader";

export default function RequestsPage() {
  return (
    <>
      <PageHeader
        title="Service Requests"
        description="Create and manage service requests."
      />
      <Card title="No data to show yet">
        <p>
          The API currently exposes create, status-transition, and history
          endpoints, but no list endpoint. Once a list endpoint exists, this
          page will render requests via <code>lib/api/requests.ts</code>.
        </p>
      </Card>
    </>
  );
}
