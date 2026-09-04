const LINK_COLUMNS = [
  {
    heading: "About ResQio",
    links: ["Objectives", "Verification process", "Data sources", "RTI"],
  },
  {
    heading: "For citizens",
    links: ["Search resources", "Report an issue", "Helpline: 1070", "FAQs"],
  },
  {
    heading: "For providers",
    links: ["Register", "Provider guidelines", "Renew verification"],
  },
  {
    heading: "Policies",
    links: ["Terms of use", "Privacy policy", "Accessibility statement", "Hyperlinking policy"],
  },
];

export default function Footer() {
  return (
    <footer className="border-t border-ink-border bg-ink text-paper/80">
      <div className="mx-auto grid max-w-6xl gap-10 px-6 py-12 sm:grid-cols-2 md:grid-cols-4">
        {LINK_COLUMNS.map((col) => (
          <div key={col.heading}>
            <h3 className="text-sm font-semibold text-paper">{col.heading}</h3>
            <ul className="mt-3 flex flex-col gap-2 text-sm">
              {col.links.map((link) => (
                <li key={link}>
                  <a href="#" className="hover:text-paper hover:underline">
                    {link}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>

      <div className="border-t border-ink-border">
        <div className="mx-auto flex max-w-6xl flex-col gap-2 px-6 py-4 text-xs text-paper/60 sm:flex-row sm:items-center sm:justify-between">
          <span>
            © {new Date().getFullYear()} ResQio. Content owned and maintained
            by the Disaster Resource Portal team.
          </span>
          <span>Last updated: {new Date().toLocaleDateString("en-IN")}</span>
        </div>
      </div>
    </footer>
  );
}
