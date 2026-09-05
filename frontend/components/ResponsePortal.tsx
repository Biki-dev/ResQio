"use client";

import { useEffect, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { ExternalLink, MapPin } from "lucide-react";
import {
  getAssistanceRequests,
  getDistributionCamps,
  getMutualAidItems,
  getResources,
  getRoadHazards,
  getSession,
  type SessionData,
  submitAssistanceRequest,
  submitRoadHazard,
  trackAssistanceRequest,
  uploadPhotoWithMulter,
} from "@/lib/api";
import type {
  AssistanceRequestResponse,
  DistributionCamp,
  MutualAidItemResponse,
  ResourceResponse,
  RoadHazardResponse,
} from "@/types";

const initialIssue = { name: "", phone_number: "", description: "", photo_url: "" };
const initialNeed = {
  name: "",
  phone_number: "",
  things_needed: "",
  amount: "1",
  description: "",
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
  const [camps, setCamps] = useState<DistributionCamp[]>([]);
  const [issue, setIssue] = useState(initialIssue);
  const [issuePhoto, setIssuePhoto] = useState<File | null>(null);
  const [need, setNeed] = useState(initialNeed);
  const [trackingCode, setTrackingCode] = useState("");
  const [trackedRequest, setTrackedRequest] = useState<AssistanceRequestResponse | null>(null);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  async function refreshFeeds() {
    setLoading(true);
    const [issueResult, requestResult, aidResult, resourceResult, campResult] = await Promise.all([
      getRoadHazards(),
      getAssistanceRequests(),
      getMutualAidItems(),
      getResources(),
      getDistributionCamps(),
    ]);
    if (issueResult.success) setIssues(issueResult.data);
    if (requestResult.success) setRequests(requestResult.data);
    if (aidResult.success) setAidItems(aidResult.data);
    if (resourceResult.success) setResources(resourceResult.data);
    if (campResult.success) setCamps(campResult.data);
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
      let photoURL = issue.photo_url;
      if (issuePhoto) {
        const uploadResult = await uploadPhotoWithMulter(issuePhoto);
        if (!uploadResult.success) throw new Error(uploadResult.error);
        photoURL = uploadResult.data.url;
      }
      const result = await submitRoadHazard({
        ...issue,
        photo_url: photoURL,
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
      setIssuePhoto(null);
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

        <section className="mb-10" aria-labelledby="help-centers-title">
          <div className="flex flex-col gap-2 border-b border-ink-border/25 pb-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-signal-dark">Public aid network</p>
              <h2 id="help-centers-title" className="mt-1 font-display text-2xl text-ink">Nearby help centers</h2>
              <p className="mt-1 text-sm text-slate">Find food, clothing, water, and other supplies currently distributed by verified organizations.</p>
            </div>
            <span className="text-xs text-slate">{camps.length} active location{camps.length === 1 ? "" : "s"}</span>
          </div>
          {camps.length === 0 ? (
            <div className="mt-5 border border-dashed border-ink-border/40 bg-paper px-5 py-8 text-center text-sm text-slate">No public distribution centers have been published yet.</div>
          ) : (
            <div className="mt-5 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {camps.map((camp) => (
                <article key={camp.id} className="border border-ink-border/25 bg-paper p-5 shadow-sm">
                  <div className="flex items-start justify-between gap-3">
                    <div><span className="text-[10px] font-bold uppercase tracking-wider text-verified">Verified aid center</span><h3 className="mt-1 font-display text-lg font-bold text-ink">{camp.camp_name}</h3></div>
                    <MapPin className="h-5 w-5 flex-shrink-0 text-signal-dark" />
                  </div>
                  <p className="mt-3 text-sm text-ink">{camp.items_available}</p>
                  <dl className="mt-4 space-y-2 text-xs text-slate"><div><dt className="font-semibold text-ink">NGO</dt><dd>{camp.provider_name}</dd></div><div><dt className="font-semibold text-ink">Location</dt><dd>{camp.address_text}</dd></div><div><dt className="font-semibold text-ink">Distribution hours</dt><dd>{camp.distribution_start.slice(0, 5)} - {camp.distribution_end.slice(0, 5)}</dd></div>{camp.contact_phone && <div><dt className="font-semibold text-ink">Contact</dt><dd>{camp.contact_phone}</dd></div>}</dl>
                  <a href={`https://www.openstreetmap.org/?mlat=${camp.latitude}&mlon=${camp.longitude}#map=17/${camp.latitude}/${camp.longitude}`} target="_blank" rel="noreferrer" className="mt-4 inline-flex items-center gap-1.5 text-xs font-semibold text-signal-dark underline underline-offset-2"><ExternalLink className="h-3.5 w-3.5" /> Open location</a>
                </article>
              ))}
            </div>
          )}
        </section>

        <div className="grid gap-6 lg:grid-cols-2">
          <form onSubmit={handleIssueSubmit} className="border border-ink-border/25 bg-paper p-6">
            <h3 className="font-display text-2xl text-ink">Report an issue</h3>
            <div className="mt-5 grid gap-4 sm:grid-cols-2">
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Your name</span><input required value={issue.name} onChange={(e) => setIssue({ ...issue, name: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
              <label className="flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Phone number</span><input required value={issue.phone_number} onChange={(e) => setIssue({ ...issue, phone_number: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
            </div>
            <label className="mt-4 flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">What did you observe?</span><textarea required rows={4} value={issue.description} onChange={(e) => setIssue({ ...issue, description: e.target.value })} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
            <label className="mt-4 flex flex-col gap-1.5"><span className="text-sm font-medium text-ink">Issue photo</span><input type="file" accept="image/jpeg,image/png,image/webp" onChange={(e) => setIssuePhoto(e.target.files?.[0] || null)} className="border border-dashed border-ink-border/40 bg-paper-dim px-3 py-3 text-sm" /><span className="text-xs text-slate">Optional. The image is stored locally and classified for responder prioritization.</span></label>
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
            <div className="mt-4 flex flex-wrap items-end gap-4"><p className="max-w-md text-xs text-slate">Priority is calculated automatically from what is needed, vulnerability details, and nearby hazard reports.</p><button disabled={submitting} className="bg-verified px-5 py-3 text-sm font-semibold text-paper disabled:opacity-50">{submitting ? "Sending..." : "Request help nearby"}</button></div>
          </form>
        </div>

        {message && <p role="status" className="mt-5 border-l-4 border-signal bg-paper px-4 py-3 text-sm text-ink">{message}</p>}

        <form onSubmit={handleTracking} className="mt-10 flex flex-col gap-3 border-y border-ink-border/25 py-6 sm:flex-row sm:items-end">
          <label className="flex flex-1 flex-col gap-1.5"><span className="text-sm font-medium text-ink">Track an assistance request</span><input required placeholder="REQ-XXXXXXXX" value={trackingCode} onChange={(e) => setTrackingCode(e.target.value)} className="border border-ink-border/30 bg-paper px-3 py-2 text-sm" /></label>
          <button className="border border-ink bg-ink px-5 py-2.5 text-sm font-semibold text-paper">Check status</button>
        </form>
        {trackedRequest && (
          <div className="mt-4 rounded-xl border border-ink-border/40 bg-paper p-6 shadow-sm">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-ink-border/20 pb-4">
              <div>
                <span className="text-xs uppercase tracking-wider text-slate">Request Tracking ID</span>
                <h4 className="font-mono text-xl font-bold text-ink">{trackedRequest.tracking_code}</h4>
              </div>

              {/* Status Badge */}
              <div className="flex items-center gap-2">
                {trackedRequest.dispatch_status === "MATCHED" ? (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/15 px-3.5 py-1 text-xs font-bold text-emerald-600 border border-emerald-500/30">
                    <span className="h-2 w-2 rounded-full bg-emerald-500"></span>
                    MATCHED & DISPATCHED
                  </span>
                ) : trackedRequest.dispatch_status === "DISPATCHING" ? (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-500/15 px-3.5 py-1 text-xs font-bold text-blue-600 border border-blue-500/30">
                    <span className="h-2 w-2 animate-ping rounded-full bg-blue-500"></span>
                    CONTACTING SUPPLIERS...
                  </span>
                ) : trackedRequest.dispatch_status === "EXHAUSTED" ? (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-red-500/15 px-3.5 py-1 text-xs font-bold text-red-600 border border-red-500/30">
                    <span className="h-2 w-2 rounded-full bg-red-500"></span>
                    ESCALATED TO OPERATORS
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-500/15 px-3.5 py-1 text-xs font-bold text-amber-600 border border-amber-500/30">
                    <span className="h-2 w-2 rounded-full bg-amber-500"></span>
                    {trackedRequest.status}
                  </span>
                )}
              </div>
            </div>

            {/* Matched Details / Handshake Code */}
            {trackedRequest.dispatch_status === "MATCHED" && (
              <div className="mt-5 rounded-lg border border-emerald-500/40 bg-emerald-500/10 p-5">
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                  <div>
                    <span className="text-xs font-semibold uppercase tracking-wider text-emerald-600">Assigned Provider</span>
                    <p className="font-display text-lg font-bold text-ink">
                      {trackedRequest.matched_provider_name || "Verified Local Responder"}
                    </p>
                    {trackedRequest.matched_provider_phone && (
                      <p className="text-xs text-slate mt-0.5">
                        Contact:{" "}
                        <a
                          href={`tel:${trackedRequest.matched_provider_phone}`}
                          className="font-semibold text-emerald-600 hover:underline"
                        >
                          {trackedRequest.matched_provider_phone}
                        </a>
                      </p>
                    )}
                  </div>

                  {trackedRequest.handshake_code && (
                    <div className="flex flex-col items-start sm:items-end">
                      <span className="text-xs font-semibold uppercase tracking-wider text-emerald-600">
                        Delivery Handshake Code
                      </span>
                      <div className="mt-1 rounded-md border border-emerald-500 bg-paper px-4 py-1.5 font-mono text-xl font-black tracking-widest text-emerald-600 shadow-sm">
                        {trackedRequest.handshake_code}
                      </div>
                      <span className="text-[10px] text-slate mt-1">Show this code upon physical handover</span>
                    </div>
                  )}
                </div>
              </div>
            )}

            {trackedRequest.dispatch_status === "EXHAUSTED" && (
              <div className="mt-4 rounded-lg border border-red-500/30 bg-red-500/10 p-4 text-xs text-slate">
                <p className="font-semibold text-red-600">
                  ⚠ All immediate automated local inventory was exhausted.
                </p>
                <p className="mt-1">
                  Your request has been escalated to regional relief coordinators with top priority for manual supplier routing.
                </p>
              </div>
            )}

            <div className="mt-4 grid gap-3 text-xs text-slate sm:grid-cols-3">
              <div>
                <span className="font-medium text-ink">Category:</span> {trackedRequest.category}
              </div>
              <div>
                <span className="font-medium text-ink">Quantity:</span> {trackedRequest.quantity_needed} units
              </div>
              <div>
                <span className="font-medium text-ink">Priority:</span> {trackedRequest.priority}
              </div>
            </div>
            {trackedRequest.description && (
              <p className="mt-2 text-xs text-slate">
                <span className="font-medium text-ink">Notes:</span> {trackedRequest.description}
              </p>
            )}
          </div>
        )}

        <div className="mt-12 grid gap-8 md:grid-cols-2">
          <Feed title="Recent issues" loading={loading}>
            {issues.slice(0, 5).map((item) => <li key={item.id}>
              <strong>{item.predicted_class || item.hazard_type.replaceAll("_", " ")}</strong>
              <span>{item.description || "Location reported"}</span>
              <small>Priority {item.priority_score.toFixed(1)}{item.confidence_score ? ` · ${(item.confidence_score * 100).toFixed(0)}% confidence` : ""}{item.cluster_size > 1 ? ` · ${item.cluster_size} nearby reports` : ""}</small>
            </li>)}
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
