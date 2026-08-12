"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ApiError, createServiceRequest } from "../../../lib/api";
import type { RequestPriority } from "../../../lib/types/api";
import { Button } from "../../../components/Button";
import { Card } from "../../../components/Card";
import { EquipmentSelect } from "../../../components/EquipmentSelect";
import { PageHeader } from "../../../components/PageHeader";

type FormState = {
  equipment_id: string;
  title: string;
  description: string;
  priority: RequestPriority;
};

const INITIAL: FormState = {
  equipment_id: "",
  title: "",
  description: "",
  priority: "medium",
};

export default function NewRequestPage() {
  const router = useRouter();
  const [form, setForm] = useState<FormState>(INITIAL);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof FormState, string>>>({});
  const [apiError, setApiError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
    setFieldErrors((prev) => {
      if (!prev[key]) return prev;
      const next = { ...prev };
      delete next[key];
      return next;
    });
  }

  function validate(values: FormState): Partial<Record<keyof FormState, string>> {
    const errors: Partial<Record<keyof FormState, string>> = {};
    if (!values.equipment_id) {
      errors.equipment_id = "Select a piece of equipment.";
    }
    if (!values.title.trim()) {
      errors.title = "Title is required.";
    }
    if (!values.description.trim()) {
      errors.description = "Description is required.";
    }
    return errors;
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setApiError(null);

    const errors = validate(form);
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    setSubmitting(true);
    try {
      const created = await createServiceRequest({
        equipment_id: form.equipment_id,
        title: form.title.trim(),
        description: form.description.trim(),
        priority: form.priority,
      });
      router.push(`/requests/${created.id}`);
    } catch (err) {
      setApiError(
        err instanceof ApiError
          ? err.message
          : "Unable to create the request. Please try again.",
      );
      setSubmitting(false);
    }
  }

  return (
    <>
      <PageHeader
        title="New Service Request"
        description="Report an issue with a piece of equipment."
      >
        <Link href="/requests">
          <Button variant="secondary">Back to requests</Button>
        </Link>
      </PageHeader>

      <Card>
        <form className="form" onSubmit={handleSubmit} noValidate>
          {apiError ? <p className="alert alert--error">{apiError}</p> : null}

          <EquipmentSelect
            value={form.equipment_id}
            onChange={(equipmentId) => update("equipment_id", equipmentId)}
            disabled={submitting}
          />
          {fieldErrors.equipment_id ? (
            <p className="field__error">{fieldErrors.equipment_id}</p>
          ) : null}

          <div className="field">
            <label className="field__label" htmlFor="title">
              Title
            </label>
            <input
              id="title"
              className="field__input"
              type="text"
              placeholder="e.g. MRI not starting"
              value={form.title}
              onChange={(e) => update("title", e.target.value)}
              disabled={submitting}
            />
            {fieldErrors.title ? (
              <p className="field__error">{fieldErrors.title}</p>
            ) : null}
          </div>

          <div className="field">
            <label className="field__label" htmlFor="description">
              Description
            </label>
            <textarea
              id="description"
              className="field__input"
              rows={4}
              placeholder="What is wrong and how does it affect operations?"
              value={form.description}
              onChange={(e) => update("description", e.target.value)}
              disabled={submitting}
            />
            {fieldErrors.description ? (
              <p className="field__error">{fieldErrors.description}</p>
            ) : null}
          </div>

          <div className="field">
            <label className="field__label" htmlFor="priority">
              Priority
            </label>
            <select
              id="priority"
              className="field__input"
              value={form.priority}
              onChange={(e) => update("priority", e.target.value as RequestPriority)}
              disabled={submitting}
            >
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="critical">Critical</option>
            </select>
          </div>

          <div className="form__actions">
            <Button variant="primary" type="submit" disabled={submitting}>
              {submitting ? "Creating…" : "Create request"}
            </Button>
          </div>
        </form>
      </Card>
    </>
  );
}
