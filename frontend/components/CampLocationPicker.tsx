"use client";

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { MapContainer, Marker, TileLayer, useMapEvents } from "react-leaflet";

interface CampLocationPickerProps {
  latitude: number | null;
  longitude: number | null;
  onChange: (latitude: number, longitude: number) => void;
}

const markerIcon = L.divIcon({
  className: "camp-location-marker",
  html: '<span style="display:block;width:22px;height:22px;border:3px solid white;border-radius:50% 50% 50% 0;background:#dc2626;transform:rotate(-45deg);box-shadow:0 2px 6px rgba(0,0,0,.35)"><i style="display:block;width:6px;height:6px;margin:5px;background:white;border-radius:50%"></i></span>',
  iconSize: [22, 22],
  iconAnchor: [11, 22],
});

function LocationEvents({ onChange }: Pick<CampLocationPickerProps, "onChange">) {
  useMapEvents({
    click(event) {
      onChange(event.latlng.lat, event.latlng.lng);
    },
  });
  return null;
}

export default function CampLocationPicker({ latitude, longitude, onChange }: CampLocationPickerProps) {
  const hasLocation = latitude !== null && longitude !== null;
  const selectedLatitude = latitude ?? 0;
  const selectedLongitude = longitude ?? 0;
  const center: [number, number] = hasLocation ? [selectedLatitude, selectedLongitude] : [20.5937, 78.9629];

  return (
    <div className="overflow-hidden rounded border border-ink-border/40">
      <div className="border-b border-ink-border/20 bg-paper-dim px-3 py-2 text-xs text-slate">
        Click the map or drag the marker to set the camp location.
      </div>
      <MapContainer center={center} zoom={hasLocation ? 14 : 5} scrollWheelZoom className="h-64 w-full">
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <LocationEvents onChange={onChange} />
        {hasLocation && (
          <Marker
            position={[selectedLatitude, selectedLongitude]}
            icon={markerIcon}
            draggable
            eventHandlers={{ dragend: (event) => {
              const position = event.target.getLatLng();
              onChange(position.lat, position.lng);
            } }}
          />
        )}
      </MapContainer>
      <div className="bg-paper px-3 py-2 text-xs text-slate">
        {hasLocation ? `Selected: ${selectedLatitude.toFixed(5)}, ${selectedLongitude.toFixed(5)}` : "No location selected yet"}
      </div>
    </div>
  );
}
