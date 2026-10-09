import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { IconChevronLeft, IconChevronRight } from "@/components/icons";
import { getUseCase, USE_CASES } from "@/lib/use-cases";
import "../../landing.css";

type Props = { params: Promise<{ slug: string }> };

export function generateStaticParams() {
  return USE_CASES.map((c) => ({ slug: c.slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const uc = getUseCase(slug);
  if (!uc) return { title: "Use case · CronCompose" };
  return {
    title: `${uc.title} · CronCompose`,
    description: uc.summary,
  };
}

export default async function UseCaseDetailPage({ params }: Props) {
  const { slug } = await params;
  const uc = getUseCase(slug);
  if (!uc) notFound();

  const idx = USE_CASES.findIndex((c) => c.slug === slug);
  const prev = idx > 0 ? USE_CASES[idx - 1] : null;
  const next = idx >= 0 && idx < USE_CASES.length - 1 ? USE_CASES[idx + 1] : null;

  return (
    <div className="public-page">
      <Link href="/use-cases" className="back-link">
        <IconChevronLeft /> All use cases
      </Link>

      <header className="public-page-hero">
        <span className="uc-badge">{uc.badge}</span>
        <h1>{uc.title}</h1>
        <p className="public-page-lede">{uc.summary}</p>
        <div className="cluster" style={{ marginTop: 18 }}>
          <Link href="/login" className="button">Get started now</Link>
          <Link href="/docs" className="button secondary">Read the docs</Link>
        </div>
      </header>

      <div className="uc-detail-grid">
        <section className="panel">
          <h2>The problem</h2>
          <p className="subtle" style={{ margin: 0, fontSize: 15, lineHeight: 1.6 }}>{uc.problem}</p>
        </section>
        <section className="panel">
          <h2>How CronCompose helps</h2>
          <p className="subtle" style={{ margin: 0, fontSize: 15, lineHeight: 1.6 }}>{uc.solution}</p>
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <h2>What you get</h2>
        <ul className="uc-outcome-list">
          {uc.outcomes.map((o) => (
            <li key={o}>{o}</li>
          ))}
        </ul>
      </section>

      <nav className="uc-pager" aria-label="More use cases">
        {prev ? (
          <Link href={`/use-cases/${prev.slug}`} className="uc-pager-link">
            <span className="subtle">Previous</span>
            <strong><IconChevronLeft /> {prev.title}</strong>
          </Link>
        ) : <span />}
        {next ? (
          <Link href={`/use-cases/${next.slug}`} className="uc-pager-link next">
            <span className="subtle">Next</span>
            <strong>{next.title} <IconChevronRight /></strong>
          </Link>
        ) : <span />}
      </nav>
    </div>
  );
}
