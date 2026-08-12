"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, listEquipment } from "../lib/api";
import type { Equipment } from "../lib/types/api";
import { Button } from "./Button";

function equipmentLabel(item: Equipment): string {
  const name = item.name || item.asset_tag || "Unnamed equipment";
  return item.asset_tag ? `${name} — ${item.asset_tag}` : name;
}

type Props = {
  value: string;
  onChange: (equipmentId: string) => void;
  disabled?: boolean;
};

export function EquipmentSelect({ value, onChange, disabled }: Props) {
  const [items, setItems] = useState<Equipment[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setItems(null);
    setError(null);
    try {
      const data = await listEquipment();
      setItems(data.equipment);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : "Unable to load equipment. Please try again.",
      );
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (error) {
    return (
      <div className="field">
        <label className="field__label">Equipment</label>
        <p className="alert alert--error">{error}</p>
        <Button variant="secondary" onClick={load}>
          Retry
        </Button>
      </div>
    );
  }

  if (items === null) {
    return (
      <div className="field">
        <label className="field__label">Equipment</label>
        <p className="loading">Loading equipment…</p>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="field">
        <label className="field__label">Equipment</label>
        <p className="empty-state">
          No equipment registered for this tenant yet.
        </p>
      </div>
    );
  }

  return (
    <div className="field">
      <label className="field__label" htmlFor="equipment_id">
        Equipment
      </label>
      <select
        id="equipment_id"
        className="field__input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
      >
        <option value="">Select equipment…</option>
        {items.map((item) => (
          <option key={item.id} value={item.id}>
            {equipmentLabel(item)}
          </option>
        ))}
      </select>
    </div>
  );
}
