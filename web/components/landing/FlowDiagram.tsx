"use client";

import { useEffect, useState } from "react";
import { IconZap } from "@/components/icons";

const STEPS = [
  { title: "Connect servers", sub: "Enroll agents with one command" },
  { title: "Import & build", sub: "Pull repos, run your pipeline" },
  { title: "Deploy processes", sub: "PM2, systemd, or Docker" },
  { title: "Schedule jobs", sub: "Cron that survives offline" },
  { title: "Observe & iterate", sub: "Logs, ports, connectors" },
];

export function FlowDiagram() {
  const [active, setActive] = useState(0);

  useEffect(() => {
    const id = window.setInterval(() => {
      setActive((i) => (i + 1) % STEPS.length);
    }, 2200);
    return () => window.clearInterval(id);
  }, []);

  return (
    <div className="flow-card" aria-hidden={false}>
      <div className="flow-steps">
        {STEPS.map((step, i) => (
          <div key={step.title} className={`flow-step${i === active ? " active" : ""}`}>
            <span className="n">{String(i + 1).padStart(2, "0")}</span>
            <div>
              <div className="t">{step.title}</div>
              <div className="s">{step.sub}</div>
            </div>
          </div>
        ))}
      </div>
      <div className="flow-float">
        <span className="ico"><IconZap /></span>
        <p>
          Agents keep firing locally even when the control plane is unreachable —
          sync resumes the moment they reconnect.
        </p>
      </div>
    </div>
  );
}
