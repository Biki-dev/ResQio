"use client";

import { useEffect, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  getAssistanceRequests,
  getMutualAidItems,
  getResources,
  getRoadHazards,
  getSession,
  type SessionData,
  submitAssistanceRequest,
  submitRoadHazard,
  trackAssistanceRequest,
} from "@/lib/api";
import type {
  AssistanceRequestResponse,
  MutualAidItemResponse,
  ResourceResponse,
  RoadHazardResponse,
} from "@/types";

const initialIssue = { name: "", phone_number: "", description: "" };
const initialNeed = {
  name: "",
  phone_number: "",
  things_needed: "",
  amount: "1",
  description: "",
  priority: "MEDIUM",
};

function requestDeviceLocation() {
  return new Promise<{ latitude: number; longitude: number }>((resolve, reject) => {
    if (!navigator.geolocation) {
      reject(new Error("Geolocation is not supported by this browser."));
      return;
    }
    navigator.geolocation.getCurrentPosition(
      ({ coords }) => resolve({ latitude: coords.latitude, longitude: coords.longitude }),
      () => reject(new Error("Location permission is required for this report.")),
      { enableHighAccuracy: true, timeout: 10000 }
    );
  });
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-IN", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value)
  );
}

export default function ResponsePortal() {
  const [session, setSession] = useState<SessionData | null>(null);
  const [issues, setIssues] = useState<RoadHazardResponse[]>([]);
  const [requests, setRequests] = useState<AssistanceRequestResponse[]>([]);
  const [aidItems, setAidItems] = useState<MutualAidItemResponse[]>([]);
  const [resources, setResources] = useState<ResourceResponse[]>([]);
  const [issue, setIssue] = useState(initialIssue);
  const [need, setNeed] = useState(initialNeed);
  const [trackingCode, setTrackingCode] = useState("");
  const [trackedRequest, setTrackedRequest] = useState<AssistanceRequestResponse | null>(null);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  async function refreshFeeds() {
    setLoading(true);
    const [issueResult, requestResult, aidResult, resourceResult] = await Promise.all([
      getRoadHazards(),
      getAssistanceRequests(),
      getMutualAidItems(),
      getResources(),
    ]);
    if (issueResult.success) setIssues(issueResult.data);
    if (requestResult.success) setRequests(requestResult.data);
    if (aidResult.success) setAidItems(aidResult.data);
    if (resourceResult.success) setResources(resourceResult.data);
    setLoading(false);
  }

  useEffect(() => {
    void refreshFeeds();
    const currentSession = getSession();
    setSession(currentSession);
    if (currentSession) {
      if (currentSession.fullName) {
        setNeed((prev) => ({ ...prev, name: prev.name || currentSession.fullName || "" }));
        setIssue((prev) => ({ ...prev, name: prev.name || currentSession.fullName || "" }));
      }
      if (currentSession.phone) {
        setNeed((prev) => ({ ...prev, phone_number: prev.phone_number || currentSession.phone || "" }));
        setIssue((prev) => ({ ...prev, phone_number: prev.phone_number || currentSession.phone || "" }));
      }
    }
  }, []);

  async function handleIssueSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setMessage("");
    try {
      const location = await requestDeviceLocation();
      const result = await submitRoadHazard({
        ...issue,
        hazard_type: "ROAD_INCIDENT",
        severity: "MEDIUM",
        ...location,
      });
      if (!result.success) throw new Error(result.error);
      setIssue((prev) => ({
        ...initialIssue,
        name: session?.fullName || "",
        phone_number: session?.phone || "",
      }));
      setMessage("Issue submitted. Thank you for helping responders verify the situation.");
      await refreshFeeds();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to submit the issue.");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleNeedSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("");

    if (!session) {
      setMessage("Authentication required: You must be signed in with a VICTIM account to submit an emergency assistance request. Please sign in or register.");
      return;
    }

    if (session.role?.toUpperCase() !== "VICTIM") {
      setMessage(`Access restricted: Your current account role is '${session.role || session.accountType}'. Only users registered with the 'VICTIM' role can submit assistance calls.`);
      return;
    }

    setSubmitting(true);
    try {
      const location = await requestDeviceLocation();
      const result = await submitAssistanceRequest({
        ...need,
        amount: Number(need.amount),
        ...location,
      });
      if (!result.success) throw new Error(result.error);
      setNeed({
        ...initialNeed,
        name: session?.fullName || "",
        phone_number: session?.phone || "",
      });
      setMessage(`Assistance request registered successfully! Tracking code: ${result.data.tracking_code}`);
      await refreshFeeds();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to register the request.");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleTracking(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("");
    const result = await trackAssistanceRequest(trackingCode.trim());
    if (result.success) setTrackedRequest(result.data);
    else setMessage(result.error);
  }

  return (
    <section id="response-portal" className="border-b border-ink-border bg-paper-dim">
      <div className="mx-auto max-w-6xl px-6 py-16">
        <div className="mb-10 max-w-2xl">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-verified">Live response desk</p>
          <h2 className="mt-2 font-display text-3xl text-ink">Report a situation or ask for help</h2>
          <p className="mt-3 leading-7 text-slate">
            Share your location so nearby responders can act quickly. You can submit anonymously and track a request with its code.
          </p>
        </div>

        <div className="grid gap-6 lg:grid-cols-2">
          <form onSubmit={handleIssueSubmit} className="border border-ink-border/25 bg-paper p-6">
            <h3 className="font-display text-2xl text-ink">Report an issue</h3>
            <div className="mt-5 grid gap-4 sm:grid-cols-2">
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Your name</span><input required value={issue.name} onChange={(e) => setIssue({ ...issue, name: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Phone number</span><input required value={issue.phone_number} onChange={(e) => setIssue({ ...issue, phone_number: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
            </div>
            <label className="mt-4 flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">What did you observe?</span><textarea required rows={4} value={issue.description} onChange={(e) => setIssue({ ...issue, description: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
            <button disabled={submitting} className="mt-5 bg-signal px-5 py-3 text-sm font-semibold text-ink disabled:opacity-50">{submitting ? "Sending..." : "Share issue location"}</button>
          </form>

          <form onSubmit={handleNeedSubmit} className="border border-ink-border/25 bg-paper p-6">
            <div className="flex items-start justify-between gap-2">
              <div>
                <h3 className="font-display text-2xl text-ink">Ask for assistance</h3>
                <p className="mt-1 text-xs text-slate">
                  Emergency aid desk. Restricted to verified <strong className="text-ink">VICTIM</strong> accounts.
                </p>
              </div>
            </div>

            {!session ? (
              <div className="mt-3 rounded border border-signal/60 bg-signal/15 p-3 text-xs text-ink">
                <div className="font-semibold text-ink">Sign-in Required</div>
                <p className="mt-0.5 text-slate">
                  You must be signed in with a VICTIM account to request immediate assistance.
                </p>
                <div className="mt-2.5 flex items-center gap-2">
                  <a href="/login" className="rounded bg-signal px-3 py-1 font-semibold text-ink hover:bg-signal-dark">
                    Sign in
                  </a>
                  <a href="/register/user" className="rounded border border-ink/30 px-3 py-1 font-medium text-ink hover:bg-paper-dim">
                    Register as Victim
                  </a>
                </div>
              </div>
            ) : session.role?.toUpperCase() === "VICTIM" ? (
              <div className="mt-3 flex items-center gap-2 rounded border border-verified/40 bg-verified/10 px-3 py-2 text-xs text-ink">
                <span className="font-semibold text-verified">✓ Verified Victim Account:</span>
                <span>{session.fullName || session.phone}</span>
              </div>
            ) : (
              <div className="mt-3 rounded border border-alert/40 bg-alert/10 p-3 text-xs text-alert">
                <div className="font-semibold">Role Restriction</div>
                <p className="mt-0.5">
                  Signed in as <strong>{session.role || session.accountType}</strong>. Emergency assistance requests require an account with the <strong>VICTIM</strong> role.
                </p>
                <a href="/register/user" className="mt-1.5 inline-block font-semibold underline">
                  Register as Victim
                </a>
              </div>
            )}

            <div className="mt-5 grid gap-4 sm:grid-cols-2">
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Your name</span><input required value={need.name} onChange={(e) => setNeed({ ...need, name: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Phone number</span><input required value={need.phone_number} onChange={(e) => setNeed({ ...need, phone_number: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">What is needed?</span><input required value={need.things_needed} onChange={(e) => setNeed({ ...need, things_needed: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Quantity</span><input required min="1" type="number" value={need.amount} onChange={(e) => setNeed({ ...need, amount: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
            </div>
            <label className="mt-4 flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Details</span><textarea rows={3} value={need.description} onChange={(e) => setNeed({ ...need, description: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
            <div className="mt-4 flex flex-wrap items-end gap-4"><label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Priority</span><select value={need.priority} onChange={(e) => setNeed({ ...need, priority: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm"><option>LOW</option><option>MEDIUM</option><option>HIGH</option><option>CRITICAL</option></select></label><button disabled={submitting} className="bg-verified px-5 py-3 text-sm font-semibold text-paper disabled:opacity-50">{submitting ? "Sending..." : "Request help nearby"}</button></div>
          </form>
        </div>

        {message && <p role="status" className="mt-5 border-l-4 border-signal bg-paper px-4 py-3 text-sm text-ink">{message}</p>}

        <form onSubmit={handleTracking} className="mt-10 flex flex-col gap-3 border-y border-ink-border/25 py-6 sm:flex-row sm:items-end">
          <label className="flex flex-1 flex-col gap-1.5"><span className="text-sm font-medium text-ink">Track an assistance request</span><input required placeholder="REQ-XXXXXXXX" value={trackingCode} onChange={(e) => setTrackingCode(e.target.value)} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
          <button className="border border-ink bg-ink px-5 py-2.5 text-sm font-semibold text-paper">Check status</button>
        </form>
        {trackedRequest && <div className="border border-verified/30 bg-verified/10 p-4 text-sm text-ink"><strong>{trackedRequest.tracking_code}</strong> is <strong>{trackedRequest.status}</strong> with {trackedRequest.category.toLowerCase()} support requested for {trackedRequest.quantity_needed}.</div>}

        <div className="mt-12 grid gap-8 md:grid-cols-2">
          <Feed title="Recent issues" loading={loading}>
            {issues.slice(0, 5).map((item) => <li key={item.id}><strong>{item.hazard_type.replaceAll("_", " ")}</strong><span>{item.description || "Location reported"}</span><small>{formatDate(item.created_at)}</small></li>)}
          </Feed>
          <Feed title="Assistance requests" loading={loading}>
            {requests.slice(0, 5).map((item) => <li key={item.id}><strong>{item.category}</strong><span>{item.quantity_needed} requested · {item.status}</span><small>{formatDate(item.created_at)}</small></li>)}
          </Feed>
          <Feed title="Community aid" loading={loading}>
            {aidItems.slice(0, 5).map((item) => <li key={item.id}><strong>{item.item_name}</strong><span>{item.quantity} available</span><small>{item.is_claimed ? "Claimed" : "Available"}</small></li>)}
          </Feed>
          <Feed title="Provider resources" loading={loading}>
            {resources.slice(0, 5).map((item) => <li key={item.id}><strong>{item.title}</strong><span>{item.current_capacity} {item.unit || "units"} · {item.category}</span><small>{item.status}</small></li>)}
          </Feed>
        </div>
      </div>
    </section>
  );
}

function Feed({ title, loading, children }: { title: string; loading: boolean; children: ReactNode }) {
  return <div><div className="flex items-baseline justify-between border-b border-ink pb-2"><h3 className="font-display text-xl text-ink">{title}</h3>{loading && <span className="text-xs text-slate">Updating...</span>}</div><ul className="divide-y divide-ink-border/20">{children}</ul></div>;
}
