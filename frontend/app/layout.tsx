import type { Metadata } from "next";
import { Noto_Serif, Noto_Sans } from "next/font/google";
import "./globals.css";

const display = Noto_Serif({
  subsets: ["latin"],
  weight: ["600", "700"],
  variable: "--font-display",
});

const body = Noto_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-body",
});

export const metadata: Metadata = {
  title: "ResQio — Verified Disaster Resource Portal",
  description:
    "An official portal for verified shelter, medical, and supply information during disasters, connecting registered providers with the public.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={`${display.variable} ${body.variable}`}>
      <body>
        <a href="#main-content" className="skip-link">
          Skip to main content
        </a>
        {children}
      </body>
    </html>
  );
}
