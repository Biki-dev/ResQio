"use client";

import "leaflet/dist/leaflet.css";
import { Circle, CircleMarker, MapContainer, Popup, TileLayer, useMap } from "react-leaflet";
import { useEffect, useMemo } from "react";
import type { RoadHazardResponse } from "@/types";

interface AdminHazardMapProps { hazards: RoadHazardResponse[] }

function Viewport({ hazards }: AdminHazardMapProps) {
  const map = useMap();
  const signature = hazards.map((item) => `${item.latitude},${item.longitude}`).join("|");
  useEffect(() => {
    const points = hazards.filter((item) => item.latitude !== undefined && item.longitude !== undefined).map((item) => [item.latitude!, item.longitude!] as [number, number]);
    if (points.length) map.fitBounds(points, { padding: [32, 32], maxZoom: 14 });
  }, [map, signature, hazards]);
  return null;
}

function colorFor(item: RoadHazardResponse) {
  if (item.severity.toUpperCase() === "CRITICAL" || item.priority_score >= 6) return "#dc2626";
  if (item.severity.toUpperCase() === "HIGH" || item.priority_score >= 4) return "#f59e0b";
  return "#2563eb";
}

export default function AdminHazardMap({ hazards }: AdminHazardMapProps) {
  const mapped = hazards.filter((item) => item.latitude !== undefined && item.longitude !== undefined);
  const center: [number, number] = mapped.length ? [mapped[0].latitude!, mapped[0].longitude!] : [20.5937, 78.9629];
  const clusters = useMemo(() => {
    const grouped = new Map<string, RoadHazardResponse>();
    mapped.filter((item) => item.cluster_id).forEach((item) => { if (!grouped.has(item.cluster_id!)) grouped.set(item.cluster_id!, item); });
    return Array.from(grouped.values()).filter((item) => item.cluster_size > 1);
  }, [mapped]);

  return <div className="overflow-hidden border border-ink-border/25 bg-paper"><div className="border-b border-ink-border/20 px-4 py-3"><h2 className="font-display text-xl font-bold text-ink">Location-wise issue map</h2><p className="mt-1 text-xs text-slate">Every mapped issue is shown from its submitted location. Red and amber pins indicate higher priority.</p></div><MapContainer center={center} zoom={mapped.length ? 11 : 5} scrollWheelZoom className="h-[480px] w-full"><TileLayer attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>' url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" /><Viewport hazards={mapped} />{clusters.map((item) => <Circle key={item.cluster_id} center={[item.latitude!, item.longitude!]} radius={150 + Math.min(item.cluster_size, 10) * 60} pathOptions={{ color: colorFor(item), fillColor: colorFor(item), fillOpacity: 0.16, weight: 2 }}><Popup>{item.cluster_size} reports in this hazard cluster</Popup></Circle>)}{mapped.map((item) => <CircleMarker key={item.id} center={[item.latitude!, item.longitude!]} radius={7} pathOptions={{ color: "#fff", weight: 2, fillColor: colorFor(item), fillOpacity: 0.95 }}><Popup><strong>{item.predicted_class || item.hazard_type}</strong><br />Severity: {item.severity}<br />Priority: {item.priority_score.toFixed(1)}<br />{item.description || "No description"}<br />Cluster reports: {item.cluster_size}</Popup></CircleMarker>)}</MapContainer>{mapped.length === 0 && <p className="border-t border-ink-border/20 px-4 py-3 text-xs text-slate">No issue has valid coordinates yet. New reports submitted with location permission will appear here.</p>}</div>;
}
