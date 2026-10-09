import type { Metadata } from "next";
import "./architecture.css";

export const metadata: Metadata = {
  title: "Architecture · CronCompose",
  description: "High-level runtime architecture: operator deploy path, trust boundaries, and core components.",
};

export default function ArchitecturePage() {
  return (
    <div className="architecture-page">
      <header className="architecture-hero">
        <p className="architecture-eyebrow">Runtime map</p>
        <h1>Architecture</h1>
        <p className="architecture-lede">
          Public marketing surface, signed-in deploy path, and mTLS agents. Cards cover PublicChrome vs AppShell, qualified process names, and connector process logs.
        </p>
      </header>
      <div className="architecture-frame">
        <iframe
          className="architecture-iframe"
          title="CronCompose runtime architecture"
          src="/app/architecture/runtime.html?embed=1"
          loading="lazy"
        />
      </div>
    </div>
  );
}
