"use client";

import { useEffect, useState, type FormEvent, type ChangeEvent } from "react";
import PageHeader from "@/components/PageHeader";
import {
  clearSession,
  createResource,
  deleteResource,
  getProviderMe,
  getProviderResources,
  getSession,
  type SessionData,
  updateResource,
  uploadPhotoWithMulter,
} from "@/lib/api";
import type { Provider, ResourceResponse } from "@/types";
import {
  Building2,
  CheckCircle2,
  Edit2,
  Image as ImageIcon,
  Loader2,
  LogOut,
  Package,
  PlusCircle,
  RefreshCw,
  Trash2,
  Upload,
  X,
} from "lucide-react";

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
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [form, setForm] = useState<ProductFormState>(INITIAL_FORM);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

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
