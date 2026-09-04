"use client";

import { useState, type FormEvent } from "react";
import PageHeader from "@/components/PageHeader";
import { FormField, FormSelect } from "@/components/ui/FormField";
import { loginProvider, loginUser } from "@/lib/api";
import type { LoginProviderRequest, LoginUserRequest } from "@/types";

type AccountType = "user" | "provider";

export default function LoginPage() {
  const [accountType, setAccountType] = useState<AccountType>("user");
  const [userForm, setUserForm] = useState<LoginUserRequest>({ phone: "", password: "" });
  const [providerForm, setProviderForm] = useState<LoginProviderRequest>({ email: "", ph_no: "", password: "" });
  const [status, setStatus] = useState<"idle" | "submitting" | "done" | "error">("idle");
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus("submitting");
    setError(null);

    const result = accountType === "user"
      ? await loginUser(userForm)
      : await loginProvider(providerForm);

    if (!result.success) {
      setStatus("error");
      setError(result.error);
      return;
    }

    setStatus("done");
  }

  return (
    <>
      <PageHeader breadcrumb={["Sign in"]} />
      <main className="mx-auto min-h-screen max-w-lg px-6 py-16">
        <h1 className="font-display text-3xl text-ink">Sign in</h1>
        <p className="mt-2 text-sm text-slate">
          Access your ResQio account and verified profile.
        </p>

        {status === "done" ? (
          <div className="mt-8 border border-verified/40 bg-verified/10 p-5 text-sm text-ink">
            <p className="font-semibold">Sign-in successful.</p>
            <a href="/admin" className="mt-3 inline-block text-verified underline">
              Continue to your account
            </a>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="mt-8 flex flex-col gap-5">
            <FormSelect
              label="Account type"
              name="account_type"
              value={accountType}
              onChange={(event) => setAccountType(event.target.value as AccountType)}
              options={[
                { value: "user", label: "Public user" },
                { value: "provider", label: "Provider" },
              ]}
            />

            {accountType === "user" ? (
              <FormField
                label="Phone number"
                name="phone"
                type="tel"
                required
                value={userForm.phone}
                onChange={(event) => setUserForm({ ...userForm, phone: event.target.value })}
              />
            ) : (
              <>
                <FormField
                  label="Email"
                  name="email"
                  type="email"
                  value={providerForm.email}
                  onChange={(event) => setProviderForm({ ...providerForm, email: event.target.value })}
                />
                <FormField
                  label="Or phone number"
                  name="ph_no"
                  type="tel"
                  value={providerForm.ph_no}
                  onChange={(event) => setProviderForm({ ...providerForm, ph_no: event.target.value })}
                />
              </>
            )}

            <FormField
              label="Password"
              name="password"
              type="password"
              required
              minLength={8}
              value={accountType === "user" ? userForm.password : providerForm.password}
              onChange={(event) => {
                if (accountType === "user") setUserForm({ ...userForm, password: event.target.value });
                else setProviderForm({ ...providerForm, password: event.target.value });
              }}
            />

            {error && <p className="text-sm text-alert">{error}</p>}

            <button
              type="submit"
              disabled={status === "submitting"}
              className="mt-2 rounded bg-signal px-6 py-3 text-sm font-medium text-ink transition-colors hover:bg-signal-dark disabled:opacity-60"
            >
              {status === "submitting" ? "Signing in..." : "Sign in"}
            </button>
          </form>
        )}
      </main>
    </>
  );
}
