"use client";

import { useState } from "react";
import { Landmark, Phone, ShieldCheck } from "lucide-react";

interface NavbarProps {
  onGetStartedClick: () => void;
}

const FONT_STEPS = ["87.5%", "100%", "112.5%"] as const;

export default function Navbar({ onGetStartedClick }: NavbarProps) {
  const [fontStep, setFontStep] = useState(1);

  function applyFontSize(step: number) {
    const clamped = Math.max(0, Math.min(FONT_STEPS.length - 1, step));
    setFontStep(clamped);
    document.documentElement.style.fontSize = FONT_STEPS[clamped];
  }

  return (
    <header className="sticky top-0 z-40">
      {/* Top strip: identity + accessibility controls, standard on
          government portals */}
      <div className="bg-ink-deep text-paper/80">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 px-6 py-1.5 text-xs">
         
          <div className="flex items-center gap-4 justify-end">
            
            <div className="flex items-center gap-1" aria-label="Adjust text size">
              <span className="mr-1 hidden sm:inline">Text size:</span>
              <button
                type="button"
                onClick={() => applyFontSize(fontStep - 1)}
                aria-label="Decrease text size"
                className="border border-paper/30 px-1.5 leading-5 hover:bg-paper/10"
              >
                A-
              </button>
              <button
                type="button"
                onClick={() => applyFontSize(1)}
                aria-label="Reset text size"
                className="border border-paper/30 px-1.5 leading-5 hover:bg-paper/10"
              >
                A
              </button>
              <button
                type="button"
                onClick={() => applyFontSize(fontStep + 1)}
                aria-label="Increase text size"
                className="border border-paper/30 px-1.5 leading-5 hover:bg-paper/10"
              >
                A+
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Main identity band */}
      <div className="border-b border-ink-border bg-paper">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 py-4">
          <a href="/" className="flex items-center gap-3">
            {/*
              Placeholder mark only — swap for the authorized department
              emblem before deployment. The National Emblem itself is
              protected under the State Emblem of India Act and must not
              be used without authorization.
            */}
            <span className="flex h-11 w-11 items-center justify-center border border-ink/20 bg-ink text-paper">
              <Landmark className="h-6 w-6" strokeWidth={1.75} />
            </span>
            <span>
              <span className="block font-display text-xl font-semibold tracking-tight text-ink">
                ResQio
              </span>
              <span className="block text-xs text-slate">
                Verified Disaster Resource Portal
              </span>
            </span>
          </a>

        </div>
      </div>

      {/* Navigation band */}
      <nav className="bg-ink">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6">
          <div className="flex items-center gap-1 text-sm text-paper/85">
            <a href="/" className="px-4 py-3 hover:bg-ink-light">
              Home
            </a>
        
            <a href="/admin" className="px-4 py-3 hover:bg-ink-light">
              Admin
            </a>
          </div>
          <button
            type="button"
            onClick={onGetStartedClick}
            className="flex items-center gap-2 bg-signal px-4 py-2.5 text-sm font-semibold text-ink transition-colors hover:bg-signal-dark"
          >
            <ShieldCheck className="h-4 w-4" strokeWidth={2.25} />
            Register / Sign up
          </button>
        </div>
      </nav>
      <div className="flag-stripe" />
    </header>
  );
}
