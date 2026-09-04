"use client";

import { useEffect, useRef } from "react";
import { Building2, UserRound, X } from "lucide-react";

interface RegistrationModalProps {
  open: boolean;
  onClose: () => void;
}

export default function RegistrationModal({
  open,
  onClose,
}: RegistrationModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    if (open) document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/70 p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
      role="presentation"
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="registration-modal-title"
        className="w-full max-w-lg rounded border border-ink-border bg-paper p-6 sm:p-8"
      >
        <div className="mb-6 flex items-start justify-between">
          <div>
            <h2
              id="registration-modal-title"
              className="font-display text-2xl text-ink"
            >
              Select an account type
            </h2>
            <p className="mt-1.5 text-sm text-slate">
              Choose the option that matches what you&apos;re registering. Provider
              accounts require government ID verification.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded p-1 text-slate transition-colors hover:bg-paper-dim hover:text-ink"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex flex-col gap-3">
          {/* Option A: providers table (ORGANISATION / INDIVIDUAL) */}
          <a
            href="/register/provider"
            className="group flex items-start gap-4 rounded border border-ink-border p-4 transition-colors hover:border-verified"
          >
            <span className="rounded bg-verified/10 p-2 text-verified">
              <Building2 className="h-5 w-5" strokeWidth={2} />
            </span>
            <span className="flex flex-col">
              <span className="font-medium text-ink">
                Organization or solo provider
              </span>
              <span className="mt-0.5 text-sm text-slate">
                Register a shelter, medical team, NGO, or yourself as an
                independent responder offering verified resources.
              </span>
            </span>
          </a>

          {/* Option B: users table (default role PUBLIC) */}
          <a
            href="/register/user"
            className="group flex items-start gap-4 rounded border border-ink-border p-4 transition-colors hover:border-signal-dark"
          >
            <span className="rounded bg-signal/10 p-2 text-signal-dark">
              <UserRound className="h-5 w-5" strokeWidth={2} />
            </span>
            <span className="flex flex-col">
              <span className="font-medium text-ink">
                Public user / requester
              </span>
              <span className="mt-0.5 text-sm text-slate">
                Create an account to search for and request verified help
                near you.
              </span>
            </span>
          </a>
        </div>
      </div>
    </div>
  );
}
