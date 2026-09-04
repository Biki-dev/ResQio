import type { Metadata } from "next";
import PageHeader from "@/components/PageHeader";

export const metadata: Metadata = {
  title: "Admin — ResQio",
};

export default function AdminPage() {
  return (
    <>
      <PageHeader breadcrumb={["Admin"]} />
      <main className="flex min-h-[70vh] items-center justify-center bg-ink px-6">
        <div className="max-w-sm text-center">
          <h1 className="font-display text-2xl text-paper">Admin console</h1>
          <p className="mt-3 text-sm text-paper/60">
            Provider verification, moderation, and reporting tools live here.
            This route is a placeholder for the authenticated admin
            dashboard.
          </p>
        </div>
      </main>
    </>
  );
}
