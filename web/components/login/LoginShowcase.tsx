"use client";

import { useEffect, useState } from "react";
import {
  IconGit,
  IconJobs,
  IconServer,
  IconZap,
} from "@/components/icons";

const SLIDES = [
  {
    kicker: "Fleet ops",
    title: "One pane for every Linux host",
    body: "Enroll agents, watch health, and jump into terminals without juggling SSH configs.",
    icon: <IconServer />,
  },
  {
    kicker: "Deploys",
    title: "Import a repo. Ship a process.",
    body: "Build once, deploy to matching hosts with PM2, systemd, or Docker — then iterate with confidence.",
    icon: <IconGit />,
  },
  {
    kicker: "Jobs",
    title: "Schedules that keep their promises",
    body: "Define cron in the control plane. Agents keep firing offline-first and sync when they reconnect.",
    icon: <IconJobs />,
  },
  {
    kicker: "Offline-first",
    title: "The plane can vanish. Work continues.",
    body: "Each agent keeps its schedule locally so a control-plane blip never silently stops production jobs.",
    icon: <IconZap />,
  },
] as const;

export function LoginShowcase() {
  const [active, setActive] = useState(0);

  useEffect(() => {
    const id = window.setInterval(() => {
      setActive((i) => (i + 1) % SLIDES.length);
    }, 4500);
    return () => window.clearInterval(id);
  }, []);

  const slide = SLIDES[active] ?? SLIDES[0];

  return (
    <aside className="login-showcase" aria-roledescription="carousel" aria-label="Product highlights">
      <div className="login-showcase-glow" aria-hidden />
      <div className="login-showcase-grid" aria-hidden />

      <div className="login-showcase-inner">
        <div className="login-showcase-kicker">
          <span className="login-showcase-ico">{slide.icon}</span>
          {slide.kicker}
        </div>
        <h2 className="login-showcase-title" key={`t-${active}`}>{slide.title}</h2>
        <p className="login-showcase-body" key={`b-${active}`}>{slide.body}</p>

        <div className="login-showcase-visual" aria-hidden key={`v-${active}`}>
          <div className="login-viz-card login-viz-a" />
          <div className="login-viz-card login-viz-b" />
          <div className="login-viz-card login-viz-c">
            <span className="login-viz-dot" />
            <span className="login-viz-bar" />
            <span className="login-viz-bar short" />
          </div>
        </div>

        <div className="login-showcase-dots" role="tablist" aria-label="Slides">
          {SLIDES.map((s, i) => (
            <button
              key={s.kicker}
              type="button"
              role="tab"
              aria-selected={i === active}
              aria-label={`Show ${s.kicker}`}
              className={`login-dot${i === active ? " on" : ""}`}
              onClick={() => setActive(i)}
            />
          ))}
        </div>
      </div>
    </aside>
  );
}
