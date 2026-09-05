"use client";

import "leaflet/dist/leaflet.css";
import { Circle, CircleMarker, MapContainer, Popup, TileLayer, useMap } from "react-leaflet";
import { useEffect, useMemo } from "react";
import type { ProviderAssistanceRequest } from "@/types";

interface ProviderRequestMapProps {
  requests: ProviderAssistanceRequest[];
}

interface DemandZone {
  latitude: number;
  longitude: number;
  count: number;
  radius: number;
}

function MapViewport({ requests }: ProviderRequestMapProps) {
  const map = useMap();
  const signature = requests.map((request) => `${request.latitude},${request.longitude}`).join("|");

  useEffect(() => {
    if (requests.length === 0) return;
    const bounds = requests.map((request) => [request.latitude, request.longitude] as [number, number]);
    map.fitBounds(bounds, { padding: [36, 36], maxZoom: 13 });
  }, [map, signature, requests]);

  return null;
}

function zoneColor(count: number) {
  if (count >= 5) return "#dc2626";
  if (count >= 3) return "#f59e0b";
  return "#16a34a";
}

export default function ProviderRequestMap({ requests }: ProviderRequestMapProps) {
  const validRequests = requests.filter(
    (request) => Number.isFinite(request.latitude) && Number.isFinite(request.longitude)
  );
  const center: [number, number] = validRequests.length
    ? [validRequests[0].latitude, validRequests[0].longitude]
    : [20.5937, 78.9629];

  const zones = useMemo<DemandZone[]>(() => {
    const buckets = new Map<string, { latitude: number; longitude: number; count: number }>();
    for (const request of validRequests) {
      const latitudeBucket = Math.round(request.latitude / 0.02);
      const longitudeBucket = Math.round(request.longitude / 0.02);
      const key = `${latitudeBucket}:${longitudeBucket}`;
      const bucket = buckets.get(key) ?? { latitude: 0, longitude: 0, count: 0 };
      bucket.latitude += request.latitude;
      bucket.longitude += request.longitude;
      bucket.count += 1;
      buckets.set(key, bucket);
    }
    return Array.from(buckets.values()).map((bucket) => ({
      latitude: bucket.latitude / bucket.count,
      longitude: bucket.longitude / bucket.count,
      count: bucket.count,
      radius: 260 + Math.min(bucket.count, 10) * 90,
    }));
  }, [validRequests]);

  return (
    <div className="overflow-hidden rounded border border-ink-border/30 bg-[#dce7e5]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-ink-border/20 bg-paper px-4 py-3">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-ink">Live demand field</p>
          <p className="mt-1 text-xs text-slate">Nearby requests combine into visual demand zones.</p>
        </div>
        <div className="flex items-center gap-3 text-[10px] font-semibold uppercase tracking-wider text-slate">
          <span className="flex items-center gap-1"><i className="h-2.5 w-2.5 rounded-full bg-green-600" /> Low</span>
          <span className="flex items-center gap-1"><i className="h-2.5 w-2.5 rounded-full bg-amber-500" /> Medium</span>
          <span className="flex items-center gap-1"><i className="h-2.5 w-2.5 rounded-full bg-red-600" /> High</span>
        </div>
      </div>
      <MapContainer center={center} zoom={validRequests.length ? 11 : 5} scrollWheelZoom className="h-[420px] w-full">
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <MapViewport requests={validRequests} />
        {zones.map((zone) => (
          <Circle
            key={`${zone.latitude}-${zone.longitude}`}
            center={[zone.latitude, zone.longitude]}
            radius={zone.radius}
            pathOptions={{ color: zoneColor(zone.count), fillColor: zoneColor(zone.count), fillOpacity: 0.18, weight: 2 }}
          >
            <Popup>{zone.count} request{zone.count === 1 ? "" : "s"} in this demand zone</Popup>
          </Circle>
        ))}
        {validRequests.map((request) => (
          <CircleMarker
            key={request.id}
            center={[request.latitude, request.longitude]}
            radius={7}
            pathOptions={{ color: "#fff", weight: 2, fillColor: zoneColor(request.priority === "CRITICAL" ? 5 : request.priority === "HIGH" ? 3 : 1), fillOpacity: 0.95 }}
          >
            <Popup>
              <strong>{request.tracking_code}</strong><br />
              {request.quantity_needed} x {request.category}<br />
              {request.requester_name}
            </Popup>
          </CircleMarker>
        ))}
      </MapContainer>
      {validRequests.length === 0 && (
        <p className="border-t border-ink-border/20 bg-paper px-4 py-3 text-xs text-slate">
          Requests without usable coordinates are still available in the cards above.
        </p>
      )}
    </div>
  );
}
