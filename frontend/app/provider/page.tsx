"use client";

import { useEffect, useState, type FormEvent, type ChangeEvent } from "react";
import dynamic from "next/dynamic";
import PageHeader from "@/components/PageHeader";
import {
  acceptDispatchPing,
  clearSession,
  createResource,
  deleteResource,
  getActiveProviderPing,
  getProviderMe,
  getProviderAssistanceRequests,
  getProviderResources,
  getSession,
  rejectDispatchPing,
  type SessionData,
  updateResource,
  uploadPhotoWithMulter,
} from "@/lib/api";
import type { ActivePing, MatchResponse, Provider, ProviderAssistanceRequest, ResourceResponse } from "@/types";
import {
  AlertTriangle,
  Bell,
  Building2,
  Check,
  CheckCircle2,
  Clock,
  Copy,
  Edit2,
  Image as ImageIcon,
  Loader2,
  LogOut,
  MapPin,
  Package,
  PlusCircle,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Upload,
  X,
} from "lucide-react";

const ProviderRequestMap = dynamic(() => import("@/components/ProviderRequestMap"), {
  ssr: false,
  loading: () => <div className="h-[420px] animate-pulse bg-paper-dim" />,
});

const CATEGORIES = [
  "FOOD",
  "WATER",
  "MEDICINE",
  "SHELTER",
  "EQUIPMENT",
  "VOLUNTEER",
  "OTHER",
];

interface ProductFormState {
  title: string;
  total_capacity: number;
  category: string;
  unit: string;
  description: string;
  contact_phone: string;
  image_url: string;
}

const INITIAL_FORM: ProductFormState = {
  title: "",
  total_capacity: 10,
  category: "FOOD",
  unit: "Units",
  description: "",
  contact_phone: "",
  image_url: "",
};

export default function ProviderPage() {
  const [session, setSession] = useState<SessionData | null>(null);
  const [provider, setProvider] = useState<Provider | null>(null);
  const [items, setItems] = useState<ResourceResponse[]>([]);
  const [requests, setRequests] = useState<ProviderAssistanceRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [form, setForm] = useState<ProductFormState>(INITIAL_FORM);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Real-time dispatch ping & match state
  const [activePing, setActivePing] = useState<ActivePing | null>(null);
  const [activeMatch, setActiveMatch] = useState<MatchResponse | null>(null);
  const [remainingSecs, setRemainingSecs] = useState<number>(0);
  const [respondingPing, setRespondingPing] = useState(false);
  const [copiedHandshake, setCopiedHandshake] = useState(false);

  // Edit modal state
  const [editingItem, setEditingItem] = useState<ResourceResponse | null>(null);
  const [editForm, setEditForm] = useState<ProductFormState>(INITIAL_FORM);
  const [editPhotoPreview, setEditPhotoPreview] = useState<string | null>(null);
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editUploading, setEditUploading] = useState(false);

  useEffect(() => {
    const currentSession = getSession();
    if (!currentSession || currentSession.accountType !== "provider") {
      window.location.href = "/login";
      return;
    }

    setSession(currentSession);
    loadProviderData(currentSession);
  }, []);

  // Poll for incoming emergency dispatch pings
  useEffect(() => {
    if (!session || session.accountType !== "provider") return;

    let mounted = true;
    async function checkPing() {
      const res = await getActiveProviderPing();
      if (!mounted) return;
      if (res.success && res.data.ping) {
        setActivePing(res.data.ping);
        setRemainingSecs(res.data.ping.remaining_seconds);
      } else {
        setActivePing(null);
      }
    }

    void checkPing();
    const interval = setInterval(checkPing, 4000);
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, [session]);

  useEffect(() => {
    if (!session || session.accountType !== "provider") return;
    let mounted = true;

    async function refreshRequests() {
      const result = await getProviderAssistanceRequests();
      if (mounted && result.success) setRequests(result.data);
    }

    void refreshRequests();
    const interval = setInterval(refreshRequests, 10000);
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, [session]);

  // Local second-by-second countdown timer
  useEffect(() => {
    if (!activePing || remainingSecs <= 0) return;

    const timer = setInterval(() => {
      setRemainingSecs((prev) => {
        if (prev <= 1) {
          setActivePing(null);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [activePing, remainingSecs]);

  async function loadProviderData(currentSession: SessionData) {
    setLoading(true);
    setError(null);
    try {
      const [provResult, resourcesResult] = await Promise.all([
        getProviderMe(currentSession.token),
        currentSession.accountId
          ? getProviderResources(currentSession.accountId)
          : Promise.resolve({ success: true, data: [] as ResourceResponse[] }),
      ]);

      if (provResult.success) {
        setProvider(provResult.data);
        setForm((prev) => ({
          ...prev,
          contact_phone: provResult.data.ph_no || "",
        }));
      } else {
        setError(provResult.error);
      }

      if (resourcesResult.success) {
        setItems(resourcesResult.data);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load provider profile");
    } finally {
      setLoading(false);
    }
  }

  async function handlePhotoUpload(
    e: ChangeEvent<HTMLInputElement>,
    isEdit: boolean = false
  ) {
    const file = e.target.files?.[0];
    if (!file) return;

    if (isEdit) {
      setEditUploading(true);
    } else {
      setUploading(true);
    }
    setError(null);

    const result = await uploadPhotoWithMulter(file);

    if (isEdit) {
      setEditUploading(false);
      if (result.success) {
        setEditForm((prev) => ({ ...prev, image_url: result.data.url }));
        setEditPhotoPreview(result.data.url);
      } else {
        setError(result.error || "Photo upload failed");
      }
    } else {
      setUploading(false);
      if (result.success) {
        setForm((prev) => ({ ...prev, image_url: result.data.url }));
        setPhotoPreview(result.data.url);
      } else {
        setError(result.error || "Photo upload failed");
      }
    }
  }

  async function handleSubmitProduct(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!session || !provider) return;

    setSubmitting(true);
    setMessage(null);
    setError(null);

    try {
      const result = await createResource({
        title: form.title.trim(),
        total_capacity: Number(form.total_capacity),
        current_capacity: Number(form.total_capacity),
        category: form.category,
        unit: form.unit.trim(),
        description: form.description.trim(),
        contact_phone: form.contact_phone.trim() || provider.ph_no,
        image_url: form.image_url,
      });

      if (!result.success) {
        throw new Error(result.error);
      }

      setMessage(`Product "${result.data.title}" successfully listed in inventory!`);
      setForm({
        ...INITIAL_FORM,
        contact_phone: provider.ph_no || "",
      });
      setPhotoPreview(null);

      // Refresh items
      if (session.accountId) {
        const refreshResult = await getProviderResources(session.accountId);
        if (refreshResult.success) setItems(refreshResult.data);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to list product");
    } finally {
      setSubmitting(false);
    }
  }

  function startEdit(item: ResourceResponse) {
    setEditingItem(item);
    setEditForm({
      title: item.title,
      total_capacity: item.total_capacity,
      category: item.category,
      unit: item.unit || "Units",
      description: item.description || "",
      contact_phone: item.contact_phone || "",
      image_url: item.image_url || "",
    });
    setEditPhotoPreview(item.image_url || null);
  }

  async function handleUpdateProduct(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editingItem || !session) return;

    setEditSubmitting(true);
    try {
      const result = await updateResource(editingItem.id, {
        title: editForm.title.trim(),
        total_capacity: Number(editForm.total_capacity),
        current_capacity: Number(editForm.total_capacity),
        category: editForm.category,
        unit: editForm.unit.trim(),
        description: editForm.description.trim(),
        contact_phone: editForm.contact_phone.trim(),
        image_url: editForm.image_url,
      });

      if (!result.success) {
        throw new Error(result.error);
      }

      setMessage(`Product "${result.data.title}" updated successfully.`);
      setEditingItem(null);

      // Refresh items
      if (session.accountId) {
        const refreshResult = await getProviderResources(session.accountId);
        if (refreshResult.success) setItems(refreshResult.data);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update product");
    } finally {
      setEditSubmitting(false);
    }
  }

  async function handleDeleteProduct(id: string, title: string) {
    if (!confirm(`Are you sure you want to remove "${title}" from your product listings?`)) {
      return;
    }

    try {
      const result = await deleteResource(id);
      if (!result.success) throw new Error(result.error);

      setMessage(`Product "${title}" was removed.`);
      setItems((prev) => prev.filter((item) => item.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete product");
    }
  }

  async function handleAcceptPing(pingId: string) {
    setRespondingPing(true);
    try {
      const res = await acceptDispatchPing(pingId);
      if (res.success) {
        setActiveMatch(res.data);
        setActivePing(null);
        setMessage("Emergency dispatch accepted! Provide the handshake code upon physical delivery.");
        if (session?.accountId) {
          const refreshResult = await getProviderResources(session.accountId);
          if (refreshResult.success) setItems(refreshResult.data);
        }
      } else {
        setError(res.error);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to accept ping");
    } finally {
      setRespondingPing(false);
    }
  }

  async function handleRejectPing(pingId: string) {
    setRespondingPing(true);
    try {
      const res = await rejectDispatchPing(pingId);
      if (res.success) {
        setActivePing(null);
        setMessage("Ping declined. Request automatically cascaded to the next nearest provider.");
      } else {
        setError(res.error);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to decline ping");
    } finally {
      setRespondingPing(false);
    }
  }

  function formatTime(seconds: number) {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs < 10 ? "0" : ""}${secs}`;
  }

  function signOut() {
    clearSession();
    window.location.href = "/login";
  }

  if (loading) {
    return (
      <main className="flex min-h-[60vh] items-center justify-center bg-paper">
        <div className="flex items-center gap-2 text-sm text-slate">
          <Loader2 className="h-5 w-5 animate-spin text-signal-dark" />
          <span>Loading provider dashboard...</span>
        </div>
      </main>
    );
  }

  return (
    <>
      <PageHeader breadcrumb={["Provider", "Product Listing & Inventory"]} />

      <main className="mx-auto min-h-screen max-w-6xl px-6 py-12">
        {/* Top provider summary strip */}
        <div className="flex flex-col gap-4 border-b border-ink-border pb-8 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <span className="flex h-9 w-9 items-center justify-center rounded bg-signal/20 text-ink">
                <Building2 className="h-5 w-5 text-signal-dark" />
              </span>
              <div>
                <span className="text-xs font-semibold uppercase tracking-wider text-verified">
                  Verified Provider Account
                </span>
                <h1 className="font-display text-2xl font-bold text-ink">
                  {provider?.name || session?.fullName || "Provider"}
                </h1>
              </div>
            </div>
            <p className="mt-2 text-xs text-slate">
              Type: <span className="font-medium text-ink">{provider?.type || "ORGANISATION"}</span> · Phone:{" "}
              <span className="font-medium text-ink">{provider?.ph_no}</span> · Email:{" "}
              <span className="font-medium text-ink">{provider?.email || "N/A"}</span> · State:{" "}
              <span className="font-medium text-ink">{provider?.state}, {provider?.dist}</span>
            </p>
          </div>

          <button
            type="button"
            onClick={signOut}
            className="flex items-center gap-2 rounded border border-ink-border/40 px-3 py-2 text-xs font-semibold text-alert hover:bg-paper-dim sm:self-start"
          >
            <LogOut className="h-4 w-4" />
            Sign out
          </button>
        </div>

        {/* ACTIVE MATCH CONFIRMED MODAL / BANNER */}
        {activeMatch && (
          <div className="mt-8 rounded-xl border-2 border-emerald-500/80 bg-emerald-950/20 p-6 shadow-xl">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <span className="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-500/20 text-emerald-400">
                  <ShieldCheck className="h-6 w-6" />
                </span>
                <div>
                  <span className="text-xs font-bold uppercase tracking-wider text-emerald-400">
                    Dispatch Match Confirmed & Active
                  </span>
                  <h2 className="text-xl font-bold text-ink">Assistance Order Assigned to You</h2>
                </div>
              </div>
              <button
                onClick={() => setActiveMatch(null)}
                className="rounded p-1 text-slate hover:bg-paper-dim hover:text-ink"
                title="Dismiss banner"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="mt-5 rounded-lg border border-emerald-500/30 bg-paper p-5">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate">
                Recipient Handshake Verification Code
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-4">
                <div className="rounded-lg border border-emerald-500/60 bg-emerald-500/10 px-6 py-2.5 font-mono text-3xl font-extrabold tracking-[0.25em] text-emerald-500">
                  {activeMatch.handshake_code}
                </div>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(activeMatch.handshake_code);
                    setCopiedHandshake(true);
                    setTimeout(() => setCopiedHandshake(false), 2000);
                  }}
                  className="flex items-center gap-1.5 rounded border border-ink-border/40 bg-paper px-3 py-2 text-xs font-semibold text-ink hover:bg-paper-dim"
                >
                  {copiedHandshake ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4" />}
                  <span>{copiedHandshake ? "Copied!" : "Copy Code"}</span>
                </button>
              </div>
              <p className="mt-3 text-xs text-slate">
                Share this 6-character code with the victim or field responder upon physical handover to authenticate fulfillment.
              </p>
            </div>

            <div className="mt-4 flex justify-end">
              <button
                type="button"
                onClick={() => setActiveMatch(null)}
                className="rounded bg-emerald-600 px-5 py-2 text-xs font-bold text-white hover:bg-emerald-500"
              >
                Acknowledge & Dismiss
              </button>
            </div>
          </div>
        )}

        {/* INCOMING EMERGENCY DISPATCH PING OFFER */}
        {activePing && (
          <div className="mt-8 rounded-xl border-2 border-amber-500 bg-amber-950/15 p-6 shadow-2xl transition-all">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2.5">
                <span className="relative flex h-4 w-4">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-400 opacity-75"></span>
                  <span className="relative inline-flex h-4 w-4 rounded-full bg-red-500"></span>
                </span>
                <span className="flex items-center gap-1 text-xs font-extrabold uppercase tracking-wider text-amber-500">
                  <AlertTriangle className="h-4 w-4 text-amber-500" />
                  Urgent Proximity Match Offer
                </span>
                <span className="rounded bg-ink/10 px-2 py-0.5 text-xs font-mono font-medium text-ink">
                  {activePing.tracking_code}
                </span>
              </div>

              {/* Countdown Timer */}
              <div className="flex items-center gap-2 rounded-full border border-amber-500/50 bg-amber-500/10 px-4 py-1 text-xs font-mono font-bold text-amber-500">
                <Clock className="h-4 w-4 animate-spin text-amber-500" />
                <span>{formatTime(remainingSecs)} remaining to respond</span>
              </div>
            </div>

            <div className="mt-4 grid gap-4 rounded-lg border border-amber-500/30 bg-paper p-5 sm:grid-cols-3">
              <div>
                <span className="text-xs uppercase tracking-wide text-slate">Requested Resource</span>
                <p className="mt-1 font-display text-lg font-bold text-ink">
                  {activePing.quantity_needed} units · {activePing.category}
                </p>
              </div>

              <div>
                <span className="text-xs uppercase tracking-wide text-slate">Estimated Proximity</span>
                <p className="mt-1 flex items-center gap-1 text-sm font-semibold text-emerald-600">
                  <MapPin className="h-4 w-4 text-emerald-600" />
                  <span>{activePing.distance_km} km away ({activePing.distance_meters}m)</span>
                </p>
              </div>

              <div>
                <span className="text-xs uppercase tracking-wide text-slate">Delivery Location</span>
                <p className="mt-1 text-sm font-medium text-ink line-clamp-2">
                  {activePing.address_text || "Coordinates specified in emergency area"}
                </p>
              </div>
            </div>

            {activePing.description && (
              <div className="mt-3 text-xs text-slate">
                <span className="font-semibold text-ink">Urgency Notes:</span> {activePing.description}
              </div>
            )}

            <div className="mt-6 flex flex-wrap items-center gap-4">
              <button
                type="button"
                disabled={respondingPing}
                onClick={() => handleAcceptPing(activePing.ping_id)}
                className="flex items-center gap-2 rounded-lg bg-emerald-600 px-6 py-2.5 text-sm font-bold text-white shadow-lg transition hover:bg-emerald-500 disabled:opacity-50"
              >
                {respondingPing ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                <span>Accept & Dispatch</span>
              </button>

              <button
                type="button"
                disabled={respondingPing}
                onClick={() => handleRejectPing(activePing.ping_id)}
                className="flex items-center gap-2 rounded-lg border border-red-500/40 bg-paper px-4 py-2.5 text-sm font-semibold text-alert transition hover:bg-red-500/10 disabled:opacity-50"
              >
                <X className="h-4 w-4" />
                <span>Decline (Cascade to Next Provider)</span>
              </button>
            </div>
          </div>
        )}

        {/* Notification alerts */}
        {message && (
          <div className="mt-6 flex items-center justify-between rounded border border-verified/40 bg-verified/10 p-4 text-sm text-ink">
            <span className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-verified" />
              {message}
            </span>
            <button
              onClick={() => setMessage(null)}
              className="text-xs text-slate hover:text-ink"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )}

        {error && (
          <div className="mt-6 flex items-center justify-between rounded border border-alert/40 bg-alert/10 p-4 text-sm text-alert">
            <span>{error}</span>
            <button
              onClick={() => setError(null)}
              className="text-xs text-alert hover:underline"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )}

        <section className="mt-10" aria-labelledby="request-inbox-title">
          <div className="flex flex-col gap-3 border-b border-ink-border/30 pb-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <Bell className="h-5 w-5 text-signal-dark" />
                <h2 id="request-inbox-title" className="font-display text-xl font-bold text-ink">
                  Community Assistance Requests ({requests.length})
                </h2>
              </div>
              <p className="mt-1 text-xs text-slate">
                Review incoming needs by urgency and location while dispatch pings continue in real time.
              </p>
            </div>
            <span className="text-[10px] font-semibold uppercase tracking-wider text-slate">Auto-refreshes every 10 seconds</span>
          </div>

          <div className="mt-6 grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
            <div className="flex max-h-[520px] flex-col gap-3 overflow-y-auto pr-1">
              {requests.length === 0 ? (
                <div className="rounded border border-dashed border-ink-border/40 bg-paper p-8 text-center text-sm text-slate">
                  No assistance requests have arrived yet.
                </div>
              ) : (
                requests.map((request) => (
                  <article key={request.id} className="rounded border border-ink-border/30 bg-paper p-4 shadow-sm transition hover:border-signal-dark/50">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-mono text-xs font-bold text-ink">{request.tracking_code}</span>
                          <span className={`rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${request.priority === "CRITICAL" ? "bg-red-100 text-red-700" : request.priority === "HIGH" ? "bg-amber-100 text-amber-700" : "bg-paper-dim text-slate"}`}>
                            {request.priority}
                          </span>
                        </div>
                        <h3 className="mt-2 font-display text-lg font-bold text-ink">
                          {request.quantity_needed} x {request.category}
                        </h3>
                      </div>
                      <span className="rounded border border-ink-border/30 px-2 py-1 text-[10px] font-semibold uppercase text-slate">
                        {request.dispatch_status || request.status}
                      </span>
                    </div>
                    <p className="mt-2 text-sm text-ink">{request.description || "No additional details provided."}</p>
                    <div className="mt-3 grid gap-1 text-xs text-slate sm:grid-cols-2">
                      <span><strong className="text-ink">Requester:</strong> {request.requester_name}</span>
                      <span><strong className="text-ink">Contact:</strong> {request.contact_phone}</span>
                      <span className="flex items-center gap-1"><MapPin className="h-3.5 w-3.5" />{request.address_text || "Location pinned on map"}</span>
                      <span><strong className="text-ink">Received:</strong> {new Date(request.created_at).toLocaleString()}</span>
                    </div>
                  </article>
                ))
              )}
            </div>
            <ProviderRequestMap requests={requests} />
          </div>
        </section>

        {/* Main Grid: Form on Left/Top, Inventory on Right/Bottom */}
        <div className="mt-10 grid gap-10 lg:grid-cols-12">
          {/* Form: List New Product */}
          <div className="lg:col-span-5">
            <div className="sticky top-20 rounded border border-ink-border/30 bg-paper p-6 shadow-sm">
              <div className="flex items-center gap-2 border-b border-ink-border/20 pb-4">
                <PlusCircle className="h-5 w-5 text-signal-dark" />
                <h2 className="font-display text-xl font-bold text-ink">
                  List a Product
                </h2>
              </div>
              <p className="mt-2 text-xs text-slate">
                Add medical supplies, water, rations, or emergency equipment to your active listings.
              </p>

              <form onSubmit={handleSubmitProduct} className="mt-6 flex flex-col gap-4">
                {/* Product Name */}
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Name of the Product *
                  </span>
                  <input
                    required
                    type="text"
                    placeholder="e.g. Bottled Drinking Water (20L Cans)"
                    value={form.title}
                    onChange={(e) => setForm({ ...form, title: e.target.value })}
                    className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink placeholder:text-slate/50 focus:border-signal-dark"
                  />
                </label>

                {/* Total Count / Quantity & Unit */}
                <div className="grid grid-cols-2 gap-3">
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                      Total Count / Quantity *
                    </span>
                    <input
                      required
                      type="number"
                      min="1"
                      value={form.total_capacity}
                      onChange={(e) =>
                        setForm({ ...form, total_capacity: Math.max(1, parseInt(e.target.value) || 1) })
                      }
                      className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                    />
                  </label>

                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                      Unit
                    </span>
                    <input
                      type="text"
                      placeholder="e.g. Boxes, Bottles, Kits"
                      value={form.unit}
                      onChange={(e) => setForm({ ...form, unit: e.target.value })}
                      className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                    />
                  </label>
                </div>

                {/* Category */}
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Category *
                  </span>
                  <select
                    value={form.category}
                    onChange={(e) => setForm({ ...form, category: e.target.value })}
                    className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                  >
                    {CATEGORIES.map((cat) => (
                      <option key={cat} value={cat}>
                        {cat}
                      </option>
                    ))}
                  </select>
                </label>

                {/* Photo of the Product (Multer Upload) */}
                <div className="flex flex-col gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Photo of the Product (Multer upload)
                  </span>
                  <div className="flex flex-col gap-2">
                    <label className="flex cursor-pointer items-center justify-center gap-2 rounded border border-dashed border-ink-border/50 bg-paper-dim px-4 py-3 text-xs font-medium text-ink transition-colors hover:bg-paper">
                      <Upload className="h-4 w-4 text-slate" />
                      <span>{uploading ? "Uploading with Multer..." : "Choose photo file"}</span>
                      <input
                        type="file"
                        accept="image/*"
                        disabled={uploading}
                        onChange={(e) => handlePhotoUpload(e, false)}
                        className="hidden"
                      />
                    </label>

                    {photoPreview && (
                      <div className="relative mt-2 h-36 w-full overflow-hidden rounded border border-ink-border/30 bg-black/5">
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={photoPreview}
                          alt="Product preview"
                          className="h-full w-full object-cover"
                        />
                        <button
                          type="button"
                          onClick={() => {
                            setForm({ ...form, image_url: "" });
                            setPhotoPreview(null);
                          }}
                          className="absolute right-2 top-2 rounded bg-ink/70 p-1 text-paper hover:bg-ink"
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    )}
                  </div>
                </div>

                {/* Contact Phone */}
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Contact Phone
                  </span>
                  <input
                    type="tel"
                    value={form.contact_phone}
                    onChange={(e) => setForm({ ...form, contact_phone: e.target.value })}
                    className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                  />
                </label>

                {/* Description */}
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Product Details / Notes
                  </span>
                  <textarea
                    rows={2}
                    placeholder="e.g. Immediate dispatch ready at sector 4 storage"
                    value={form.description}
                    onChange={(e) => setForm({ ...form, description: e.target.value })}
                    className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                  />
                </label>

                <button
                  type="submit"
                  disabled={submitting || uploading}
                  className="mt-2 flex items-center justify-center gap-2 rounded bg-signal px-5 py-3 text-sm font-semibold text-ink transition-colors hover:bg-signal-dark disabled:opacity-60"
                >
                  {submitting ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      <span>Saving to database...</span>
                    </>
                  ) : (
                    <>
                      <Package className="h-4 w-4" />
                      <span>List Product in Database</span>
                    </>
                  )}
                </button>
              </form>
            </div>
          </div>

          {/* Right Column: Previous Added Data / Listed Items */}
          <div className="lg:col-span-7">
            <div className="flex items-center justify-between border-b border-ink-border/30 pb-4">
              <div className="flex items-center gap-2">
                <Package className="h-5 w-5 text-verified" />
                <h2 className="font-display text-xl font-bold text-ink">
                  Your Listed Products ({items.length})
                </h2>
              </div>
              <button
                type="button"
                onClick={() => session && loadProviderData(session)}
                className="flex items-center gap-1.5 text-xs text-slate hover:text-ink"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                Refresh
              </button>
            </div>

            {items.length === 0 ? (
              <div className="mt-8 rounded border border-ink-border/30 bg-paper p-8 text-center">
                <Package className="mx-auto h-12 w-12 text-slate/40" strokeWidth={1.5} />
                <h3 className="mt-3 font-display text-lg font-semibold text-ink">
                  No products listed yet
                </h3>
                <p className="mt-1 text-xs text-slate">
                  Use the form on the left to submit your emergency supply inventory.
                </p>
              </div>
            ) : (
              <div className="mt-6 flex flex-col gap-4">
                {items.map((item) => (
                  <div
                    key={item.id}
                    className="flex flex-col gap-4 rounded border border-ink-border/30 bg-paper p-4 transition-all hover:border-ink-border/60 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="flex items-start gap-3">
                      {item.image_url ? (
                        /* eslint-disable-next-line @next/next/no-img-element */
                        <img
                          src={item.image_url}
                          alt={item.title}
                          className="h-16 w-16 flex-shrink-0 rounded border border-ink-border/30 object-cover"
                        />
                      ) : (
                        <div className="flex h-16 w-16 flex-shrink-0 items-center justify-center rounded border border-ink-border/30 bg-paper-dim text-slate">
                          <ImageIcon className="h-6 w-6" />
                        </div>
                      )}

                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <h4 className="font-semibold text-ink">{item.title}</h4>
                          <span className="rounded bg-paper-dim px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-ink border border-ink-border/30">
                            {item.category}
                          </span>
                        </div>
                        <p className="mt-1 text-xs text-slate">
                          Total Count:{" "}
                          <strong className="text-ink font-semibold">
                            {item.total_capacity} {item.unit || "units"}
                          </strong>
                          {item.contact_phone && ` · Contact: ${item.contact_phone}`}
                        </p>
                        {item.description && (
                          <p className="mt-1 text-xs text-slate line-clamp-1">
                            {item.description}
                          </p>
                        )}
                        <p className="mt-1 text-[10px] text-slate/70">
                          ID: {item.id.slice(0, 8)}... · Status: {item.status}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center gap-2 self-end sm:self-center">
                      <button
                        type="button"
                        onClick={() => startEdit(item)}
                        className="flex items-center gap-1.5 rounded border border-ink-border/40 px-3 py-1.5 text-xs font-semibold text-ink hover:bg-paper-dim"
                      >
                        <Edit2 className="h-3.5 w-3.5" />
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDeleteProduct(item.id, item.title)}
                        className="flex items-center gap-1.5 rounded border border-alert/30 px-2.5 py-1.5 text-xs font-semibold text-alert hover:bg-alert/10"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </main>

      {/* Edit Product Modal */}
      {editingItem && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink/60 p-4">
          <div className="w-full max-w-lg rounded border border-ink-border bg-paper p-6 shadow-xl">
            <div className="flex items-center justify-between border-b border-ink-border/30 pb-3">
              <h3 className="font-display text-lg font-bold text-ink">
                Edit Product Listing
              </h3>
              <button
                onClick={() => setEditingItem(null)}
                className="text-slate hover:text-ink"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleUpdateProduct} className="mt-4 flex flex-col gap-4">
              <label className="flex flex-col gap-1">
                <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                  Product Name *
                </span>
                <input
                  required
                  type="text"
                  value={editForm.title}
                  onChange={(e) => setEditForm({ ...editForm, title: e.target.value })}
                  className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                />
              </label>

              <div className="grid grid-cols-2 gap-3">
                <label className="flex flex-col gap-1">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Total Count / Capacity *
                  </span>
                  <input
                    required
                    type="number"
                    min="1"
                    value={editForm.total_capacity}
                    onChange={(e) =>
                      setEditForm({
                        ...editForm,
                        total_capacity: Math.max(1, parseInt(e.target.value) || 1),
                      })
                    }
                    className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                  />
                </label>

                <label className="flex flex-col gap-1">
                  <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                    Category
                  </span>
                  <select
                    value={editForm.category}
                    onChange={(e) => setEditForm({ ...editForm, category: e.target.value })}
                    className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                  >
                    {CATEGORIES.map((cat) => (
                      <option key={cat} value={cat}>
                        {cat}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              {/* Photo Update */}
              <div className="flex flex-col gap-1">
                <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                  Product Photo (Change via Multer)
                </span>
                <label className="flex cursor-pointer items-center justify-center gap-2 rounded border border-dashed border-ink-border/50 bg-paper-dim px-3 py-2 text-xs font-medium text-ink hover:bg-paper">
                  <Upload className="h-3.5 w-3.5 text-slate" />
                  <span>{editUploading ? "Uploading..." : "Upload new photo"}</span>
                  <input
                    type="file"
                    accept="image/*"
                    disabled={editUploading}
                    onChange={(e) => handlePhotoUpload(e, true)}
                    className="hidden"
                  />
                </label>

                {editPhotoPreview && (
                  <div className="relative mt-2 h-28 w-full overflow-hidden rounded border border-ink-border/30">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={editPhotoPreview}
                      alt="Edit preview"
                      className="h-full w-full object-cover"
                    />
                    <button
                      type="button"
                      onClick={() => {
                        setEditForm({ ...editForm, image_url: "" });
                        setEditPhotoPreview(null);
                      }}
                      className="absolute right-2 top-2 rounded bg-ink/70 p-1 text-paper hover:bg-ink"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                )}
              </div>

              <label className="flex flex-col gap-1">
                <span className="text-xs font-semibold uppercase tracking-wider text-ink">
                  Details / Notes
                </span>
                <textarea
                  rows={2}
                  value={editForm.description}
                  onChange={(e) =>
                    setEditForm({ ...editForm, description: e.target.value })
                  }
                  className="rounded border border-ink-border/40 bg-paper px-3 py-2 text-sm text-ink focus:border-signal-dark"
                />
              </label>

              <div className="mt-2 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setEditingItem(null)}
                  className="rounded border border-ink-border px-4 py-2 text-xs font-medium text-ink hover:bg-paper-dim"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={editSubmitting || editUploading}
                  className="flex items-center gap-2 rounded bg-signal px-4 py-2 text-xs font-semibold text-ink hover:bg-signal-dark disabled:opacity-60"
                >
                  {editSubmitting ? "Updating..." : "Save Changes"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
