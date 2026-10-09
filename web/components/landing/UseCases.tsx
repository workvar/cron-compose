"use client";

import { useState } from "react";
import Link from "next/link";
import { IconChevronRight } from "@/components/icons";

const CASES = [
  {
    id: "fleet",
    tab: "Fleet ops",
    badge: "Servers",
    title: "One pane for every Linux host",
    body: "Enroll agents, watch online status, and jump into terminals without juggling SSH configs or bastion hopscotch.",
    href: "/servers",
    cta: "Browse servers",
  },
  {
    id: "deploy",
    tab: "Deploys",
    badge: "Ship",
    title: "Import a repo. Ship a process.",
    body: "Configure build steps, env, and process managers once — then redeploy with confidence across matching hosts.",
    href: "/deploys",
    cta: "Open deploys",
  },
  {
    id: "jobs",
    tab: "Jobs",
    badge: "Cron",
    title: "Schedules that keep their promises",
    body: "Define cron once in the control plane. Agents execute offline-first and report results when they can.",
    href: "/jobs",
    cta: "View jobs",
  },
  {
    id: "connect",
    tab: "Connectors",
    badge: "Ops",
    title: "Act on running processes",
    body: "Restart, inspect logs, and manage PM2, systemd, or Docker objects without leaving CronCompose.",
    href: "/connectors",
    cta: "Open connectors",
  },
] as const;

export function UseCases() {
  const [active, setActive] = useState(0);
  const current = CASES[active] ?? CASES[0];

  return (
    <div>
      <div className="uc-tabs" role="tablist" aria-label="Product use cases">
        {CASES.map((c, i) => (
          <button
            key={c.id}
            type="button"
            role="tab"
            aria-selected={i === active}
            className={`uc-tab${i === active ? " on" : ""}`}
            onClick={() => setActive(i)}
          >
            {c.tab}
          </button>
        ))}
      </div>

      <div className="uc-panel" role="tabpanel">
        <div className="uc-copy">
          <span className="uc-badge">{current.badge}</span>
          <h3>{current.title}</h3>
          <p>{current.body}</p>
          <Link href={current.href} className="uc-link">
            {current.cta} <IconChevronRight />
          </Link>
        </div>
        <div className="uc-visual" aria-hidden>
          <UseCaseVisual index={active} />
        </div>
      </div>
    </div>
  );
}

function UseCaseVisual({ index }: { index: number }) {
  const hues = [160, 172, 148, 186];
  const hue = hues[index % hues.length];
  return (
    <svg className="uc-visual-diagram" viewBox="0 0 280 210" fill="none">
      <rect x="24" y="28" width="100" height="64" rx="14" fill={`hsla(${hue},70%,55%,0.14)`} stroke={`hsla(${hue},70%,60%,0.55)`} />
      <rect x="140" y="28" width="116" height="40" rx="12" fill="rgba(255,255,255,0.04)" stroke="rgba(255,255,255,0.12)" />
      <rect x="140" y="80" width="116" height="40" rx="12" fill="rgba(255,255,255,0.04)" stroke="rgba(255,255,255,0.12)" />
      <rect x="24" y="108" width="232" height="72" rx="16" fill="rgba(255,255,255,0.03)" stroke="rgba(255,255,255,0.1)" />
      <circle cx="52" cy="144" r="10" fill={`hsla(${hue},80%,60%,0.9)`} />
      <rect x="74" y="136" width="120" height="8" rx="4" fill="rgba(255,255,255,0.18)" />
      <rect x="74" y="152" width="84" height="6" rx="3" fill="rgba(255,255,255,0.1)" />
      <path
        d="M74 60h40M164 48h60M164 100h40"
        stroke={`hsla(${hue},70%,65%,0.7)`}
        strokeWidth="2"
        strokeLinecap="round"
        className="curve-path"
        style={{ strokeDasharray: "none", strokeDashoffset: 0 }}
      />
    </svg>
  );
}
