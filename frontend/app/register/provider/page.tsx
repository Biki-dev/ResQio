"use client";

import { useState, type FormEvent } from "react";
import { FormField, FormSelect } from "@/components/ui/FormField";
import PageHeader from "@/components/PageHeader";
import { registerProvider } from "@/lib/api";
import { ProviderType, type RegisterProviderRequest } from "@/types";

const INITIAL_STATE: RegisterProviderRequest = {
  type: ProviderType.ORGANISATION,
  name: "",
  authorized_person: "",
  govt_id: "",
  email: "",
  ph_no: "",
  state: "",
  dist: "",
  location: "",
  password: "",
};

export default function ProviderRegistrationPage() {
  const [form, setForm] = useState<RegisterProviderRequest>(INITIAL_STATE);
  const [status, setStatus] = useState<"idle" | "submitting" | "error" | "done">(
    "idle"
  );
  const [error, setError] = useState<string | null>(null);

  function update<K extends keyof RegisterProviderRequest>(
    key: K,
    value: RegisterProviderRequest[K]
  ) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setStatus("submitting");
    setError(null);

    const result = await registerProvider(form);

    if (result.success) {
      setStatus("done");
    } else {
      setStatus("error");
      setError(result.error ?? "Registration failed. Please try again.");
    }
  }

  if (status === "done") {
    return (
      <>
        <PageHeader breadcrumb={["Register", "Provider"]} />
        <main className="flex min-h-[60vh] items-center justify-center bg-paper px-6">
          <div className="max-w-md text-center">
            <h1 className="font-display text-2xl text-ink">
              Provider Account Created
            </h1>
            <p className="mt-3 text-sm text-slate">
              You are signed in as a provider. You can now access your provider dashboard and list emergency products in inventory.
            </p>
            <div className="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3">
              <a
                href="/provider"
                className="rounded bg-signal px-5 py-2.5 text-sm font-semibold text-ink transition-colors hover:bg-signal-dark"
              >
                Go to Provider Dashboard & Inventory
              </a>
            </div>
          </div>
        </main>
      </>
    );
  }

  return (
    <>
      <PageHeader breadcrumb={["Register", "Provider"]} />
      <main className="mx-auto min-h-screen max-w-lg px-6 py-16">
        <h1 className="font-display text-3xl text-ink">
          Register as a provider
        </h1>
      <p className="mt-2 text-sm text-slate">
        For organizations, NGOs, medical teams, or individuals offering
        verified resources directly.
      </p>

      <form onSubmit={handleSubmit} className="mt-8 flex flex-col gap-5">
        <FormSelect
          label="Provider type"
          name="type"
          value={form.type}
          onChange={(e) => update("type", e.target.value as ProviderType)}
          options={[
            { value: ProviderType.ORGANISATION, label: "Organisation" },
            { value: ProviderType.INDIVIDUAL, label: "Individual" },
          ]}
        />

        <FormField
          label={form.type === ProviderType.ORGANISATION ? "Organisation name" : "Full name"}
          name="name"
          required
          maxLength={255}
          value={form.name}
          onChange={(e) => update("name", e.target.value)}
        />

        {form.type === ProviderType.ORGANISATION && (
          <FormField
            label="Authorized person"
            name="authorized_person"
            maxLength={255}
            value={form.authorized_person ?? ""}
            onChange={(e) => update("authorized_person", e.target.value)}
          />
        )}

        <FormField
          label="Government ID"
          name="govt_id"
          required
          maxLength={50}
          value={form.govt_id}
          onChange={(e) => update("govt_id", e.target.value)}
        />

        <FormField
          label="Email"
          name="email"
          type="email"
          required
          maxLength={255}
          value={form.email}
          onChange={(e) => update("email", e.target.value)}
        />

        <FormField
          label="Phone number"
          name="ph_no"
          type="tel"
          required
          maxLength={20}
          value={form.ph_no}
          onChange={(e) => update("ph_no", e.target.value)}
        />

        <div className="grid grid-cols-2 gap-4">
          <FormField
            label="State"
            name="state"
            required
            maxLength={255}
            value={form.state}
            onChange={(e) => update("state", e.target.value)}
          />
          <FormField
            label="District"
            name="dist"
            required
            maxLength={255}
            value={form.dist}
            onChange={(e) => update("dist", e.target.value)}
          />
        </div>

        <FormField
          label="Location"
          name="location"
          required
          maxLength={255}
          placeholder="POINT(77.2090 28.6139)"
          value={form.location}
          onChange={(e) => update("location", e.target.value)}
        />

        <FormField
          label="Password"
          name="password"
          type="password"
          required
          minLength={8}
          value={form.password}
          onChange={(e) => update("password", e.target.value)}
        />

        {error && <p className="text-sm text-alert">{error}</p>}

        <button
          type="submit"
          disabled={status === "submitting"}
          className="mt-2 rounded bg-verified px-6 py-3 text-sm font-medium text-paper transition-colors hover:bg-verified-light disabled:opacity-60"
        >
          {status === "submitting" ? "Submitting…" : "Submit for verification"}
        </button>
      </form>
      </main>
    </>
  );
}
