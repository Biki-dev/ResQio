"use client";

import { useEffect, useState } from "react";
import PageHeader from "@/components/PageHeader";
import { clearSession, getProviderMe, getSession, getUserMe } from "@/lib/api";
import type { Provider, User } from "@/types";

export default function AccountPage() {
  const [profile, setProfile] = useState<User | Provider | null>(null);
  const [accountType, setAccountType] = useState<"user" | "provider" | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const session = getSession();
    if (!session) {
      setError("You are not signed in.");
      return;
    }

    setAccountType(session.accountType);
    const request = session.accountType === "user"
      ? getUserMe(session.token)
      : getProviderMe(session.token);

    request.then((result) => {
      if (result.success) setProfile(result.data);
      else setError(result.error);
    });
  }, []);

  function signOut() {
    clearSession();
    window.location.href = "/login";
  }

  return (
    <>
      <PageHeader breadcrumb={["Account"]} />
      <main className="mx-auto min-h-screen max-w-3xl px-6 py-16">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.1em] text-verified">
              Authenticated account
            </p>
            <h1 className="mt-2 font-display text-3xl text-ink">Your profile</h1>
          </div>
          {profile && (
            <button type="button" onClick={signOut} className="border border-ink-border px-4 py-2 text-sm text-ink hover:bg-paper-dim">
              Sign out
            </button>
          )}
        </div>

        {error ? (
          <div className="mt-8 border border-alert/40 bg-alert/10 p-5 text-sm text-ink">
            <p>{error}</p>
            <a href="/login" className="mt-3 inline-block text-alert underline">Sign in</a>
          </div>
        ) : profile ? (
          <div className="mt-8 grid gap-4 border border-ink-border bg-paper p-6 sm:grid-cols-2">
            {accountType === "user" ? (
              <>
                <ProfileField label="Full name" value={(profile as User).full_name} />
                <ProfileField label="Phone" value={(profile as User).phone} />
                <ProfileField label="Role" value={(profile as User).role} />
              </>
            ) : (
              <>
                <ProfileField label="Provider" value={(profile as Provider).name} />
                <ProfileField label="Email" value={(profile as Provider).email} />
                <ProfileField label="Phone" value={(profile as Provider).ph_no} />
                <ProfileField label="Location" value={(profile as Provider).location ?? "Not provided"} />
              </>
            )}
          </div>
        ) : (
          <p className="mt-8 text-sm text-slate">Loading your profile...</p>
        )}
      </main>
    </>
  );
}

function ProfileField({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-slate">{label}</dt>
      <dd className="mt-1 text-sm font-medium text-ink">{value}</dd>
    </div>
  );
}
