"use client";

import heroIllustration from "@/images/Hero-illustration.png";

interface HeroSectionProps {
  onGetStartedClick: () => void;
}

const LIVE_FEED = [
  { label: "Community Shelter — Sector 4", meta: "Verified 6 min ago", beds: "42 beds open" },
  { label: "Field Medical Post — Riverside", meta: "Verified 11 min ago", beds: "Triage active" },
  { label: "Water & Supplies — Depot 12", meta: "Verified 19 min ago", beds: "Stock: high" },
];

export default function HeroSection({ onGetStartedClick }: HeroSectionProps) {
  return (
    <section className="border-b border-ink-border bg-paper">
      <div className="mx-auto grid max-w-6xl gap-12 px-6 py-16 md:grid-cols-[1.1fr_0.9fr] md:py-20">
        {/* Left: headline */}
        <div className="flex flex-col justify-center">
          <span className="mb-4 inline-flex w-fit items-center border border-verified/40 bg-verified/10 px-3 py-1 text-xs font-medium text-verified">
            Government-verified information service
          </span>

          <h1 className="max-w-xl font-display text-4xl leading-[1.15] text-ink sm:text-[2.75rem]">
            Verified emergency resources, for every district.
          </h1>

          <p className="mt-5 max-w-xl text-base leading-7 text-slate">
            ResQio publishes shelter, medical, and supply information only
            after it is confirmed by a registered provider or authority —
            so citizens act on facts, not rumours, during a disaster.
          </p>

          <div className="mt-8 flex flex-wrap items-center gap-4">
            <button
              type="button"
              onClick={onGetStartedClick}
              className="border border-signal bg-signal px-6 py-3 text-sm font-semibold text-ink transition-colors hover:bg-signal-dark"
            >
              Register a resource
            </button>
          
          </div>
        </div>

        {/* Right: live verification panel */}
        <div className="self-center">
           <img
            src={heroIllustration.src}
            alt="Illustration of a disaster response team verifying resources"
            className="w-full"
          />
        </div>
      </div>
    </section>
  );
}
