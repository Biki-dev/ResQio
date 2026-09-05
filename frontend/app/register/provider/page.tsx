"use client";

import { useState, useEffect, useCallback, type FormEvent } from "react";
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

interface GeoState {
  status: "idle" | "requesting" | "granted" | "denied";
  lat?: number;
  lon?: number;
  accuracy?: number;
  error?: string;
}

export default function ProviderRegistrationPage() {
  const [form, setForm] = useState<RegisterProviderRequest>(INITIAL_STATE);
  const [geo, setGeo] = useState<GeoState>({ status: "requesting" });
  const [status, setStatus] = useState<"idle" | "submitting" | "error" | "done">(
    "idle"
  );
  const [error, setError] = useState<string | null>(null);

  const requestLocation = useCallback(() => {
    if (!navigator.geolocation) {
      setGeo({
        status: "denied",
        error: "Geolocation is not supported by your browser.",
      });
      return;
    }

    setGeo({ status: "requesting" });
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const lat = pos.coords.latitude;
        const lon = pos.coords.longitude;
        const pointStr = `POINT(${lon.toFixed(6)} ${lat.toFixed(6)})`;
        setGeo({
          status: "granted",
          lat,
          lon,
          accuracy: Math.round(pos.coords.accuracy),
        });
        setForm((prev) => ({
          ...prev,
          location: pointStr,
          latitude: lat,
          longitude: lon,
        }));
      },
      (err) => {
        setGeo({
          status: "denied",
          error: err.message || "Location permission was denied.",
        });
      },
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 0 }
    );
  }, []);

  useEffect(() => {
    requestLocation();
  }, [requestLocation]);

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

    if (!form.location) {
      setStatus("error");
      setError("Please allow location permission to anchor your facility coordinates for dispatch matching.");
      return;
    }

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
              You are signed in as a verified relief provider. Your headquarters coordinates have been automatically registered for proximity dispatch.
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

          {/* Automatic Geolocation Detection Badge - replaces manual POINT(...) field */}
          <div className="rounded-lg border border-ink-border bg-paper-dim p-4">
            {geo.status === "requesting" && (
              <div className="flex items-center gap-3">
                <span className="relative flex h-3 w-3">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-signal opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-3 w-3 bg-signal"></span>
                </span>
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wider text-slate">Facility Coordinates</p>
                  <p className="text-xs text-ink mt-0.5">Capturing dispatch location automatically…</p>
                </div>
              </div>
            )}

            {geo.status === "granted" && (
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <span className="text-verified font-bold text-lg">📍</span>
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wider text-verified">
                      Location Anchored Automatically
                    </p>
                    <p className="text-xs font-mono text-ink mt-0.5">
                      {geo.lat?.toFixed(4)}° N, {geo.lon?.toFixed(4)}° E (±{geo.accuracy}m)
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={requestLocation}
                  className="text-xs font-medium text-slate hover:text-ink underline"
                >
                  Recalibrate
                </button>
              </div>
            )}

            {geo.status === "denied" && (
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wider text-alert">
                    Location Access Denied
                  </p>
                  <p className="text-xs text-slate mt-0.5">
                    ResQio requires your GPS coordinates to dispatch emergency requests to your facility.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={requestLocation}
                  className="shrink-0 rounded bg-signal px-3 py-1.5 text-xs font-semibold text-ink hover:bg-signal-dark"
                >
                  Allow GPS
                </button>
              </div>
            )}
          </div>

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
