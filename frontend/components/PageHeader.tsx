import { Landmark } from "lucide-react";

interface PageHeaderProps {
  breadcrumb: string[];
}

export default function PageHeader({ breadcrumb }: PageHeaderProps) {
  return (
    <header>
      <div className="border-b border-ink-border bg-paper">
        <div className="mx-auto flex max-w-6xl items-center gap-3 px-6 py-4">
          <span className="flex h-9 w-9 items-center justify-center border border-ink/20 bg-ink text-paper">
            <Landmark className="h-5 w-5" strokeWidth={1.75} />
          </span>
          <a href="/" className="font-display text-lg font-semibold text-ink">
            ResQio
          </a>
        </div>
      </div>
      <nav aria-label="Breadcrumb" className="bg-ink-deep">
        <ol className="mx-auto flex max-w-6xl items-center gap-2 px-6 py-2 text-xs text-paper/70">
          <li>
            <a href="/" className="hover:text-paper hover:underline">
              Home
            </a>
          </li>
          {breadcrumb.map((crumb, i) => (
            <li key={crumb} className="flex items-center gap-2">
              <span aria-hidden="true">/</span>
              {i === breadcrumb.length - 1 ? (
                <span className="text-paper">{crumb}</span>
              ) : (
                <span>{crumb}</span>
              )}
            </li>
          ))}
        </ol>
      </nav>
      <div className="flag-stripe" />
    </header>
  );
}
