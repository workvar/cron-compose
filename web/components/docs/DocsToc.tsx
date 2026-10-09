"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

type Section = readonly [id: string, label: string];

export function DocsToc({
  sections,
}: {
  sections: readonly Section[];
}) {
  const [active, setActive] = useState(sections[0]?.[0] ?? "");

  useEffect(() => {
    const nodes = sections
      .map(([id]) => document.getElementById(id))
      .filter((el): el is HTMLElement => !!el);
    if (nodes.length === 0) return;

    const visible = new Map<string, number>();

    const pick = () => {
      // Prefer the section whose top is nearest above the sticky offset.
      let best = sections[0]?.[0] ?? "";
      let bestTop = Number.NEGATIVE_INFINITY;
      for (const el of nodes) {
        const top = el.getBoundingClientRect().top;
        if (top <= 120 && top > bestTop) {
          bestTop = top;
          best = el.id;
        }
      }
      // If nothing is above the line yet, use the first intersecting section.
      if (bestTop === Number.NEGATIVE_INFINITY) {
        for (const el of nodes) {
          const rect = el.getBoundingClientRect();
          if (rect.bottom > 120) {
            best = el.id;
            break;
          }
        }
      }
      setActive(best);
    };

    const obs = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          visible.set(entry.target.id, entry.intersectionRatio);
        }
        pick();
      },
      { rootMargin: "-100px 0px -55% 0px", threshold: [0, 0.1, 0.25, 0.5, 1] },
    );

    for (const el of nodes) obs.observe(el);
    pick();
    window.addEventListener("scroll", pick, { passive: true });
    window.addEventListener("hashchange", pick);

    return () => {
      obs.disconnect();
      window.removeEventListener("scroll", pick);
      window.removeEventListener("hashchange", pick);
    };
  }, [sections]);

  return (
    <nav className="docs-toc" aria-label="On this page">
      <div className="docs-toc-label">croncompose.yml</div>
      {sections.map(([id, label]) => (
        <a
          key={id}
          href={`#${id}`}
          className={active === id ? "active" : undefined}
          aria-current={active === id ? "location" : undefined}
        >
          {label}
        </a>
      ))}
      <div className="docs-toc-label" style={{ marginTop: 18 }}>Use it</div>
      <Link href="/deploys/new">Import a project →</Link>
    </nav>
  );
}
