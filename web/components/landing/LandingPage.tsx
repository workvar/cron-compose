"use client";

import Link from "next/link";
import {
  IconArchitecture,
  IconChevronRight,
  IconGit,
  IconJobs,
  IconKey,
  IconPlug,
  IconPorts,
  IconServer,
  IconShield,
  IconZap,
} from "@/components/icons";
import { Reveal } from "./Reveal";
import { FlowDiagram } from "./FlowDiagram";
import { UseCases } from "./UseCases";
import { EfficiencyCurve } from "./EfficiencyCurve";

const CAPABILITIES = [
  { label: "Servers", icon: <IconServer />, href: "/use-cases/fleet" },
  { label: "Deploys", icon: <IconGit />, href: "/use-cases/deploys" },
  { label: "Jobs", icon: <IconJobs />, href: "/use-cases/jobs" },
  { label: "Connectors", icon: <IconPlug />, href: "/use-cases/connectors" },
  { label: "Secrets", icon: <IconKey />, href: "/docs" },
  { label: "Ports", icon: <IconPorts />, href: "/docs" },
] as const;

const DEMANDS = [
  {
    icon: <IconServer />,
    text: "No single place to see which hosts are healthy, what they run, or whether agents are still syncing.",
  },
  {
    icon: <IconJobs />,
    text: "Cron files drift per machine. When a box goes offline, nobody knows which schedules silently stopped.",
  },
  {
    icon: <IconShield />,
    text: "Deploys, secrets, and process restarts live in five tools — so handoffs and audits get messy.",
  },
] as const;

export function LandingPage() {
  return (
    <div className="landing">
      <div className="landing-inner">
        <section className="landing-hero">
          <div>
            <Reveal>
              <div className="landing-kicker">
                <IconZap /> The control plane
              </div>
            </Reveal>
            <Reveal delay={1}>
              <h1>
                Deploy and schedule across your <span className="accent">Linux fleet</span>.
              </h1>
            </Reveal>
            <Reveal delay={2}>
              <p className="lede">
                CronCompose connects agents, ships processes, and keeps jobs running —
                even when the plane is unreachable.
              </p>
            </Reveal>
            <Reveal delay={3}>
              <div className="landing-hero-actions">
                <Link href="/login" className="button">
                  Get started now
                </Link>
                <a href="#stories" className="button secondary">
                  See use cases
                </a>
              </div>
            </Reveal>
          </div>
          <Reveal delay={2}>
            <FlowDiagram />
          </Reveal>
        </section>

        <section id="capabilities" className="landing-section" style={{ paddingTop: 0 }}>
          <div className="cap-strip">
            {CAPABILITIES.map((c, i) => (
              <Reveal key={c.label} delay={Math.min(i, 5) as 0 | 1 | 2 | 3 | 4 | 5}>
                <Link href={c.href} className="cap-card">
                  <span className="cap-ico">{c.icon}</span>
                  <span>{c.label}</span>
                </Link>
              </Reveal>
            ))}
          </div>
        </section>

        <section id="stories" className="landing-section">
          <Reveal>
            <div className="section-label">
              <IconArchitecture /> Use cases
            </div>
            <div className="section-head-row">
              <h2>
                Built for how you actually <span className="accent">operate</span>.
              </h2>
              <Link href="/use-cases" className="button secondary sm">
                All use cases <IconChevronRight />
              </Link>
            </div>
          </Reveal>
          <Reveal delay={1}>
            <UseCases />
          </Reveal>
        </section>

        <section className="landing-section">
          <Reveal>
            <div className="pain-card">
              <div className="section-label">Pain points</div>
              <h2>
                You&apos;re expected to ship <span className="accent">fast</span>, but
                traditional ops tooling can&apos;t keep up with a growing{" "}
                <span className="accent">fleet</span>.
              </h2>
            </div>
          </Reveal>
          <div className="demand-grid">
            {DEMANDS.map((d, i) => (
              <Reveal key={i} delay={(i + 1) as 1 | 2 | 3}>
                <div className="demand-card">
                  <span className="ico">{d.icon}</span>
                  <p>{d.text}</p>
                </div>
              </Reveal>
            ))}
          </div>
        </section>

        <section id="why" className="landing-section">
          <Reveal>
            <div className="response-block">
              <div className="section-label">
                <IconZap /> Our response
              </div>
              <h2>
                CronCompose gives you the leverage of a composed{" "}
                <span className="accent">ops team</span> — without the glue scripts.
              </h2>
              <p className="section-lede">
                Agents, deploys, jobs, secrets, and connectors share one control plane
                and one visual language.
              </p>

              <div className="stats-bento">
                <div className="stats-intro">
                  <span className="uc-badge" style={{ color: "var(--green)", background: "var(--green-soft)" }}>
                    Offline-first
                  </span>
                  <p style={{ margin: 0, color: "var(--text-2)", fontSize: 14, lineHeight: 1.55 }}>
                    Each agent keeps its schedule locally. When connectivity returns,
                    status and logs sync back without babysitting.
                  </p>
                  <Link href="/login" className="button">
                    Get started now
                  </Link>
                </div>
                <div className="stat-cell">
                  <strong>&lt;1 command</strong>
                  <span>Agent install and enrollment.</span>
                </div>
                <div className="stat-cell">
                  <strong>Always-on</strong>
                  <span>Jobs keep firing without the plane.</span>
                </div>
                <div className="stat-cell">
                  <strong>One plane</strong>
                  <span>Deploys, cron, secrets, connectors.</span>
                </div>
                <div className="stat-cell">
                  <strong>Linux-native</strong>
                  <span>PM2, systemd, Docker — your stack.</span>
                </div>
              </div>

              <EfficiencyCurve />
            </div>
          </Reveal>
        </section>

        <Reveal>
          <section className="landing-cta">
            <h2>
              Ready to compose your <span className="accent">fleet</span>?
            </h2>
            <p>
              Sign in to import a repo, enroll an agent, and ship your first process.
            </p>
            <div className="cluster">
              <Link href="/login" className="button">
                Get started now
              </Link>
              <Link href="/docs" className="button secondary">
                Read the docs
              </Link>
            </div>
          </section>
        </Reveal>
      </div>
    </div>
  );
}
