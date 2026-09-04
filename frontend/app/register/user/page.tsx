"use client";

import { useState, type FormEvent } from "react";
import { FormField } from "@/components/ui/FormField";
import PageHeader from "@/components/PageHeader";
import { registerUser } from "@/lib/api";
import type { RegisterUserRequest } from "@/types";

const INITIAL_STATE: RegisterUserRequest = {
  full_name: "",
  phone: "",
  password: "",
};

export default function UserRegistrationPage() {
  const [form, setForm] = useState<RegisterUserRequest>(INITIAL_STATE);
  const [status, setStatus] = useState<"idle" | "submitting" | "error" | "done">(
    "idle"
  );
  const [error, setError] = useState<string | null>(null);

  function update<K extends keyof RegisterUserRequest>(
    key: K,
    value: RegisterUserRequest[K]
  ) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setStatus("submitting");
    setError(null);

    const result = await registerUser(form);

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
        <PageHeader breadcrumb={["Register", "Public user"]} />
        <main className="flex min-h-[60vh] items-center justify-center bg-paper px-6">
          <div className="max-w-sm text-center">
            <h1 className="font-display text-2xl text-ink">You&apos;re in</h1>
            <p className="mt-3 text-sm text-slate">
              Your account is ready. You can now search for verified
              resources near you.
            </p>
          </div>
        </main>
      </>
    );
  }

  return (
    <>
      <PageHeader breadcrumb={["Register", "Public user"]} />
      <main className="mx-auto min-h-screen max-w-lg px-6 py-16">
        <h1 className="font-display text-3xl text-ink">Create your account</h1>
      <p className="mt-2 text-sm text-slate">
        For anyone searching for or requesting help during a disaster.
      </p>

      <form onSubmit={handleSubmit} className="mt-8 flex flex-col gap-5">
        <FormField
          label="Full name"
          name="full_name"
          required
          maxLength={100}
          value={form.full_name}
          onChange={(e) => update("full_name", e.target.value)}
        />

        <FormField
          label="Phone number"
          name="phone"
          type="tel"
          required
          maxLength={20}
          value={form.phone}
          onChange={(e) => update("phone", e.target.value)}
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
          className="mt-2 rounded bg-signal px-6 py-3 text-sm font-medium text-ink transition-colors hover:bg-signal-dark disabled:opacity-60"
        >
          {status === "submitting" ? "Creating account…" : "Create account"}
        </button>
      </form>
      </main>
    </>
  );
}
