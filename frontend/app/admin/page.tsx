"use client";

import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import PageHeader from "@/components/PageHeader";
import {
  clearSession,
  getAdminAlerts,
  getAdminCamps,
  getAdminAuditLogs,
  getAdminHazards,
  getAdminOverview,
  getAdminProviders,
  getAdminRequests,
  getAdminResources,
  getAdminUsers,
  getSession,
  updateAdminProviderStatus,
  updateAdminUserRole,
  assignAdminRequest,
  rebuildAdminHazardClusters,
  verifyAdminHazard,
} from "@/lib/api";
import type { AdminAuditLog, AdminOverview, AdminProvider, AdminUser, AssistanceRequestResponse, DistributionCamp, ExhaustedAlert, ResourceResponse, RoadHazardResponse } from "@/types";
import { AlertTriangle, Check, Download, History, LogOut, RefreshCw, ShieldCheck, Users } from "lucide-react";

const EMPTY_OVERVIEW: AdminOverview = { users: 0, providers: 0, open_requests: 0, critical_requests: 0, active_hazards: 0, active_camps: 0, pending_dispatches: 0, exhausted_requests: 0, embedded_requests: 0, embedded_resources: 0, hazard_clusters: 0, clustered_hazards: 0 };
const ROLE_OPTIONS = ["PUBLIC", "VICTIM", "PROVIDER", "COORDINATOR", "ADMIN"];
const AdminHazardMap = dynamic(() => import("@/components/AdminHazardMap"), { ssr: false, loading: () => <div className="h-[520px] animate-pulse bg-paper-dim" /> });

function downloadCsv<T extends Record<string, unknown>>(filename: string, rows: T[]) {
  if (!rows.length) return;
  const keys = Array.from(new Set(rows.flatMap((row) => Object.keys(row))));
  const escape = (value: unknown) => `"${String(value ?? "").replaceAll('"', '""')}"`;
  const csv = [keys.join(","), ...rows.map((row) => keys.map((key) => escape(row[key])).join(","))].join("\n");
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
  const link = document.createElement("a"); link.href = url; link.download = filename; link.click(); URL.revokeObjectURL(url);
}

function ExportBar({ count, total, onSelectAll, onExport }: { count: number; total: number; onSelectAll: () => void; onExport: () => void }) {
  return <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate"><span>{count} selected of {total}</span><div className="flex gap-2"><button type="button" onClick={onSelectAll} className="border border-ink-border/30 px-2 py-1 font-semibold text-ink">Select all</button><button type="button" onClick={onExport} disabled={!count} className="flex items-center gap-1 border border-ink-border/30 px-2 py-1 font-semibold text-ink disabled:opacity-40"><Download className="h-3 w-3" /> CSV</button></div></div>;
}

export default function AdminPage() {
  const [overview, setOverview] = useState<AdminOverview>(EMPTY_OVERVIEW);
  const [alerts, setAlerts] = useState<ExhaustedAlert[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [providers, setProviders] = useState<AdminProvider[]>([]);
  const [auditLogs, setAuditLogs] = useState<AdminAuditLog[]>([]);
  const [hazards, setHazards] = useState<RoadHazardResponse[]>([]);
  const [requests, setRequests] = useState<AssistanceRequestResponse[]>([]);
  const [resources, setResources] = useState<ResourceResponse[]>([]);
  const [camps, setCamps] = useState<DistributionCamp[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [selected, setSelected] = useState<Record<string, Set<string>>>({});
  const [assignmentProvider, setAssignmentProvider] = useState<Record<string, string>>({});

  async function loadDashboard() {
    setLoading(true);
    setError(null);
    const [overviewResult, alertsResult, usersResult, providersResult, auditResult, hazardsResult, requestsResult, resourcesResult, campsResult] = await Promise.all([
      getAdminOverview(), getAdminAlerts(), getAdminUsers(), getAdminProviders(), getAdminAuditLogs(), getAdminHazards(), getAdminRequests(), getAdminResources(), getAdminCamps(),
    ]);
    if (overviewResult.success) setOverview(overviewResult.data); else setError(overviewResult.error);
    if (alertsResult.success) setAlerts(alertsResult.data.alerts); else setError(alertsResult.error);
    if (usersResult.success) setUsers(usersResult.data); else setError(usersResult.error);
    if (providersResult.success) setProviders(providersResult.data); else setError(providersResult.error);
    if (auditResult.success) setAuditLogs(auditResult.data); else setError(auditResult.error);
    if (hazardsResult.success) setHazards(hazardsResult.data); else setError(hazardsResult.error);
    if (requestsResult.success) setRequests(requestsResult.data); else setError(requestsResult.error);
    if (resourcesResult.success) setResources(resourcesResult.data); else setError(resourcesResult.error);
    if (campsResult.success) setCamps(campsResult.data); else setError(campsResult.error);
    setLoading(false);
  }

  useEffect(() => {
    const session = getSession();
    if (!session || session.accountType !== "user" || session.role?.toUpperCase() !== "ADMIN") {
      window.location.href = "/login";
      return;
    }
    void loadDashboard();
  }, []);

  async function changeRole(user: AdminUser, role: string) {
    if (user.role === role || !window.confirm(`Change ${user.full_name}'s role to ${role}?`)) return;
    const result = await updateAdminUserRole(user.id, role);
    if (!result.success) { setError(result.error); return; }
    setUsers((previous) => previous.map((item) => item.id === user.id ? result.data : item));
    setMessage(`Updated ${user.full_name}'s role.`);
  }

  async function changeProvider(provider: AdminProvider, payload: { is_active?: boolean; is_verified?: boolean }) {
    const result = await updateAdminProviderStatus(provider.id, payload);
    if (!result.success) { setError(result.error); return; }
    setProviders((previous) => previous.map((item) => item.id === provider.id ? result.data : item));
    setMessage(`Updated ${provider.name}.`);
  }

  async function verifyHazard(hazard: RoadHazardResponse) {
    const result = await verifyAdminHazard(hazard.id, !hazard.is_verified);
    if (!result.success) { setError(result.error); return; }
    setHazards((previous) => previous.map((item) => item.id === hazard.id ? { ...item, is_verified: result.data.is_verified } : item));
    setMessage(`Issue ${result.data.is_verified ? "verified" : "unverified"}.`);
  }

  async function rebuildClusters() {
    const result = await rebuildAdminHazardClusters();
    if (!result.success) { setError(result.error); return; }
    setMessage(`${result.data.processed} hazard records processed into location/class clusters.`);
    await loadDashboard();
  }

  function toggleSelected(section: string, id: string) { setSelected((current) => { const next = new Set(current[section] || []); next.has(id) ? next.delete(id) : next.add(id); return { ...current, [section]: next }; }); }
  function selectAll(section: string, ids: string[]) { setSelected((current) => ({ ...current, [section]: new Set(ids) })); }
  function exportRows<T extends Record<string, unknown>>(section: string, filename: string, rows: T[]) { const ids = selected[section] || new Set(); downloadCsv(filename, rows.filter((row) => ids.has(String(row.id)))); }
  async function assignRequest(request: AssistanceRequestResponse) { const providerId = assignmentProvider[request.id]; if (!providerId) { setError("Select a provider first."); return; } const result = await assignAdminRequest(request.id, providerId); if (!result.success) { setError(result.error); return; } setMessage(`Assignment sent for ${request.tracking_code}. The provider will receive the normal dispatch popup.`); }

  function signOut() { clearSession(); window.location.href = "/login"; }
  const metrics = [["Users", overview.users], ["Providers", overview.providers], ["Open requests", overview.open_requests], ["Critical requests", overview.critical_requests], ["Unverified hazards", overview.active_hazards], ["Active camps", overview.active_camps], ["Pending dispatches", overview.pending_dispatches], ["Exhausted requests", overview.exhausted_requests]] as const;

  return <>
    <PageHeader breadcrumb={["Admin", "Operations dashboard"]} />
    <main className="mx-auto min-h-screen max-w-7xl px-6 py-12">
      <header className="flex flex-wrap items-start justify-between gap-4 border-b border-ink-border pb-8">
        <div><p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-verified"><ShieldCheck className="h-4 w-4" /> Restricted operations</p><h1 className="mt-2 font-display text-3xl font-bold text-ink">Admin control center</h1><p className="mt-2 max-w-2xl text-sm text-slate">Manage users, providers, safety reports, and emergency operations.</p></div>
        <div className="flex gap-2"><button type="button" onClick={() => void loadDashboard()} className="flex items-center gap-2 border border-ink-border px-3 py-2 text-xs font-semibold text-ink"><RefreshCw className={loading ? "h-4 w-4 animate-spin" : "h-4 w-4"} /> Refresh</button><button type="button" onClick={signOut} className="flex items-center gap-2 border border-alert/30 px-3 py-2 text-xs font-semibold text-alert"><LogOut className="h-4 w-4" /> Sign out</button></div>
      </header>
      {error && <div className="mt-6 border border-alert/40 bg-alert/10 p-4 text-sm text-alert">{error}</div>}
      {message && <div className="mt-6 border border-verified/40 bg-verified/10 p-4 text-sm text-ink">{message}</div>}
      <section className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">{metrics.map(([label, value]) => <div key={label} className="border border-ink-border/25 bg-paper p-5"><span className="text-[10px] font-bold uppercase tracking-wider text-signal-dark">{label}</span><p className="mt-4 font-display text-3xl font-bold text-ink">{value}</p></div>)}</section>
      <section className="mt-8 border border-ink-border/25 bg-paper p-6"><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-display text-xl font-bold text-ink">AI embeddings & clustering</h2><p className="mt-1 text-xs text-slate">Embeddings power dispatch/resource matching. Hazard clusters combine the same predicted class within 100 meters.</p></div><button type="button" onClick={() => void rebuildClusters()} className="rounded bg-signal px-4 py-2 text-xs font-bold text-ink">Rebuild hazard clusters</button></div><div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4"><div><span className="text-xs text-slate">Embedded requests</span><p className="mt-1 text-2xl font-bold text-ink">{overview.embedded_requests}</p></div><div><span className="text-xs text-slate">Embedded resources</span><p className="mt-1 text-2xl font-bold text-ink">{overview.embedded_resources}</p></div><div><span className="text-xs text-slate">Hazard clusters</span><p className="mt-1 text-2xl font-bold text-ink">{overview.hazard_clusters}</p></div><div><span className="text-xs text-slate">Clustered hazards</span><p className="mt-1 text-2xl font-bold text-ink">{overview.clustered_hazards}</p></div></div></section>
      <section className="mt-8"><AdminHazardMap hazards={hazards} /></section>

      <section className="mt-10 grid gap-8 lg:grid-cols-2">
        <div className="border border-ink-border/25 bg-paper p-6"><div className="flex items-center gap-2 border-b border-ink-border/20 pb-4"><Users className="h-5 w-5 text-signal-dark" /><h2 className="font-display text-xl font-bold text-ink">1. User profiles</h2></div><ExportBar count={selected.users?.size || 0} total={users.length} onSelectAll={() => selectAll("users", users.map((item) => item.id))} onExport={() => exportRows("users", "users.csv", users as unknown as Record<string, unknown>[])} /><div className="mt-4 max-h-[420px] overflow-auto">{users.map((user) => <div key={user.id} className="flex flex-wrap items-center justify-between gap-3 border-b border-ink-border/15 py-3"><div className="flex items-start gap-2"><input type="checkbox" checked={selected.users?.has(user.id) || false} onChange={() => toggleSelected("users", user.id)} /><div><p className="font-semibold text-ink">{user.full_name}</p><p className="text-xs text-slate">{user.phone} · {user.is_active ? "Active" : "Inactive"}</p></div></div><select value={user.role} onChange={(event) => void changeRole(user, event.target.value)} className="border border-ink-border/30 bg-paper px-2 py-1 text-xs font-semibold text-ink">{ROLE_OPTIONS.map((role) => <option key={role}>{role}</option>)}</select></div>)}</div></div>
        <div className="border border-ink-border/25 bg-paper p-6"><div className="flex items-center gap-2 border-b border-ink-border/20 pb-4"><ShieldCheck className="h-5 w-5 text-verified" /><h2 className="font-display text-xl font-bold text-ink">4. Provider profiles</h2></div><ExportBar count={selected.providers?.size || 0} total={providers.length} onSelectAll={() => selectAll("providers", providers.map((item) => item.id))} onExport={() => exportRows("providers", "providers.csv", providers as unknown as Record<string, unknown>[])} /><div className="mt-4 max-h-[420px] overflow-auto">{providers.map((provider) => <div key={provider.id} className="border-b border-ink-border/15 py-3"><div className="flex flex-wrap items-start justify-between gap-3"><div className="flex gap-2"><input type="checkbox" checked={selected.providers?.has(provider.id) || false} onChange={() => toggleSelected("providers", provider.id)} /><div><p className="font-semibold text-ink">{provider.name}</p><p className="text-xs text-slate">{provider.email} · {provider.state}, {provider.district}</p></div></div><div className="flex gap-2"><button type="button" onClick={() => void changeProvider(provider, { is_verified: !provider.is_verified })} className="flex items-center gap-1 border border-ink-border/30 px-2 py-1 text-[10px] font-bold text-ink">{provider.is_verified && <Check className="h-3 w-3" />}{provider.is_verified ? "Verified" : "Verify"}</button><button type="button" onClick={() => void changeProvider(provider, { is_active: !provider.is_active })} className="border border-alert/30 px-2 py-1 text-[10px] font-bold text-alert">{provider.is_active ? "Deactivate" : "Activate"}</button></div></div></div>)}</div></div>
      </section>

      <section className="mt-8 grid gap-8 lg:grid-cols-2"><div className="border border-ink-border/25 bg-paper p-6"><div className="flex items-center gap-2 border-b border-ink-border/20 pb-4"><AlertTriangle className="h-5 w-5 text-alert" /><h2 className="font-display text-xl font-bold text-ink">Escalated requests</h2></div>{alerts.length === 0 ? <p className="py-8 text-sm text-slate">No exhausted requests require intervention.</p> : <div className="mt-3 max-h-80 overflow-auto">{alerts.map((alert) => <article key={alert.id} className="border-b border-ink-border/15 py-3"><p className="font-semibold text-ink">{alert.tracking_code} · {alert.category}</p><p className="text-xs text-slate">{alert.description || "No additional details."}</p></article>)}</div>}</div><div className="border border-ink-border/25 bg-paper p-6"><div className="flex items-center gap-2 border-b border-ink-border/20 pb-4"><History className="h-5 w-5 text-signal-dark" /><h2 className="font-display text-xl font-bold text-ink">Audit activity</h2></div><div className="mt-3 max-h-80 overflow-auto">{auditLogs.length === 0 ? <p className="py-8 text-sm text-slate">No admin actions recorded yet.</p> : auditLogs.map((log) => <div key={log.id} className="border-b border-ink-border/15 py-3"><p className="text-xs font-semibold text-ink">{log.action} · {log.target_type}</p><p className="text-[11px] text-slate">{new Date(log.created_at).toLocaleString()}</p></div>)}</div></div></section>
      <section className="mt-8 grid gap-8 lg:grid-cols-2"><div className="border border-ink-border/25 bg-paper p-6"><h2 className="font-display text-xl font-bold text-ink">2. User issues ({hazards.length})</h2><ExportBar count={selected.hazards?.size || 0} total={hazards.length} onSelectAll={() => selectAll("hazards", hazards.map((item) => item.id))} onExport={() => exportRows("hazards", "issues.csv", hazards as unknown as Record<string, unknown>[])} /><div className="mt-3 max-h-80 overflow-auto">{hazards.map((hazard) => <div key={hazard.id} className="flex items-center justify-between gap-3 border-b border-ink-border/15 py-3"><div className="flex gap-2"><input type="checkbox" checked={selected.hazards?.has(hazard.id) || false} onChange={() => toggleSelected("hazards", hazard.id)} /><div><p className="font-semibold text-ink">{hazard.predicted_class || hazard.hazard_type}</p><p className="text-xs text-slate">{hazard.description || "No details"} · {hazard.name}</p></div></div><button type="button" onClick={() => void verifyHazard(hazard)} className="border border-ink-border/30 px-2 py-1 text-[10px] font-bold text-ink">{hazard.is_verified ? "Unverify" : "Verify"}</button></div>)}</div></div><div className="border border-ink-border/25 bg-paper p-6"><h2 className="font-display text-xl font-bold text-ink">3. Assistance requests ({requests.length})</h2><ExportBar count={selected.requests?.size || 0} total={requests.length} onSelectAll={() => selectAll("requests", requests.map((item) => item.id))} onExport={() => exportRows("requests", "assistance-requests.csv", requests as unknown as Record<string, unknown>[])} /><div className="mt-3 max-h-80 overflow-auto">{requests.map((request) => <div key={request.id} className="border-b border-ink-border/15 py-3"><div className="flex gap-2"><input type="checkbox" checked={selected.requests?.has(request.id) || false} onChange={() => toggleSelected("requests", request.id)} /><div><p className="font-semibold text-ink">{request.tracking_code} · {request.category} · {request.priority}</p><p className="text-xs text-slate">{request.requester_name} · {request.status} · {request.description || "No details"}</p><div className="mt-2 flex gap-2"><select value={assignmentProvider[request.id] || ""} onChange={(event) => setAssignmentProvider((current) => ({ ...current, [request.id]: event.target.value }))} className="border border-ink-border/30 bg-paper px-2 py-1 text-[10px] text-ink"><option value="">Assign verified provider</option>{providers.filter((provider) => provider.is_active && provider.is_verified).map((provider) => <option key={provider.id} value={provider.id}>{provider.name}</option>)}</select><button type="button" onClick={() => void assignRequest(request)} className="border border-signal-dark px-2 py-1 text-[10px] font-bold text-ink">Send dispatch popup</button></div></div></div></div>)}</div></div></section>
      <section className="mt-8 grid gap-8 lg:grid-cols-2"><div className="border border-ink-border/25 bg-paper p-6"><h2 className="font-display text-xl font-bold text-ink">5. Provider products ({resources.length})</h2><ExportBar count={selected.resources?.size || 0} total={resources.length} onSelectAll={() => selectAll("resources", resources.map((item) => item.id))} onExport={() => exportRows("resources", "provider-products.csv", resources as unknown as Record<string, unknown>[])} /><div className="mt-3 max-h-80 overflow-auto">{resources.map((resource) => <div key={resource.id} className="flex gap-2 border-b border-ink-border/15 py-3"><input type="checkbox" checked={selected.resources?.has(resource.id) || false} onChange={() => toggleSelected("resources", resource.id)} /><div><p className="font-semibold text-ink">{resource.title} · {resource.category}</p><p className="text-xs text-slate">{resource.current_capacity} {resource.unit || "units"} available · {resource.status}</p></div></div>)}</div></div><div className="border border-ink-border/25 bg-paper p-6"><h2 className="font-display text-xl font-bold text-ink">6. Provider camps ({camps.length})</h2><ExportBar count={selected.camps?.size || 0} total={camps.length} onSelectAll={() => selectAll("camps", camps.map((item) => item.id))} onExport={() => exportRows("camps", "distribution-camps.csv", camps as unknown as Record<string, unknown>[])} /><div className="mt-3 max-h-80 overflow-auto">{camps.map((camp) => <div key={camp.id} className="flex gap-2 border-b border-ink-border/15 py-3"><input type="checkbox" checked={selected.camps?.has(camp.id) || false} onChange={() => toggleSelected("camps", camp.id)} /><div><p className="font-semibold text-ink">{camp.camp_name} · {camp.provider_name}</p><p className="text-xs text-slate">{camp.address_text} · {camp.items_available}</p></div></div>)}</div></div></section>
      <section className="mt-8 border border-ink-border/25 bg-paper p-6"><div className="flex items-center justify-between gap-3 border-b border-ink-border/20 pb-4"><div><h2 className="font-display text-xl font-bold text-ink">7. Location-wise issue zones</h2><p className="mt-1 text-xs text-slate">Uses the existing PostGIS locations, predicted classes, and rebuilt 100 m clusters.</p></div><span className="text-xs text-slate">{hazards.length} reports</span></div><div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{Array.from(hazards.reduce((groups, hazard) => { const key = hazard.predicted_class || hazard.hazard_type; groups.set(key, (groups.get(key) || 0) + 1); return groups; }, new Map<string, number>())).sort((a, b) => b[1] - a[1]).map(([label, count]) => <div key={label} className="border border-ink-border/20 bg-paper-dim p-4"><p className="font-semibold text-ink">{label}</p><p className="mt-1 text-2xl font-bold text-ink">{count}</p><p className="text-xs text-slate">reports in this issue zone</p></div>)}</div></section>
    </main>
  </>;
}
