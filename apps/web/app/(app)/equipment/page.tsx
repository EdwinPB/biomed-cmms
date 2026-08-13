import { Card } from "../../../components/Card";
import { PageHeader } from "../../../components/PageHeader";

export default function EquipmentPage() {
  return (
    <>
      <PageHeader
        title="Equipment"
        description="Biomedical equipment inventory."
      />
      <Card title="Placeholder">
        <p>
          The backend has an equipment domain but no HTTP endpoints yet. This
          page will list equipment once <code>/api/v1/equipment</code> is
          available.
        </p>
      </Card>
    </>
  );
}
