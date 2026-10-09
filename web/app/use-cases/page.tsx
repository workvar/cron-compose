import type { Metadata } from "next";
import Link from "next/link";
import { IconChevronRight } from "@/components/icons";
import { USE_CASES } from "@/lib/use-cases";
import "../landing.css";

export const metadata: Metadata = {
  title: "Use cases · CronCompose",
  description: "How teams use CronCompose for fleet ops, deploys, jobs, and connectors.",
};

export default function UseCasesIndexPage() {
  return (
    <div className="public-page">
      <header className="public-page-hero">
        <p className="docs-eyebrow">Use cases</p>
        <h1>Built for how you actually operate</h1>
        <p className="public-page-lede">
          From enrolling Linux hosts to shipping processes and keeping cron honest —
          CronCompose is the control plane for day-two ops.
        </p>
      </header>

      <div className="uc-index-grid">
        {USE_CASES.map((c) => (
          <Link key={c.slug} href={`/use-cases/${c.slug}`} className="uc-index-card">
            <span className="uc-badge">{c.badge}</span>
            <h2>{c.title}</h2>
            <p>{c.summary}</p>
            <span className="uc-link">
              Read use case <IconChevronRight />
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}
