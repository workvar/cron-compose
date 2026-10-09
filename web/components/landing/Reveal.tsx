"use client";

import type { ReactNode } from "react";

/** Staggered entrance — CSS animation always finishes visible (no IO stuck state). */
export function Reveal({
  children,
  className = "",
  delay = 0,
}: {
  children: ReactNode;
  className?: string;
  delay?: 0 | 1 | 2 | 3 | 4 | 5;
}) {
  const delayClass = delay > 0 ? ` reveal-d${delay}` : "";
  return (
    <div className={`reveal reveal-animate${delayClass}${className ? ` ${className}` : ""}`}>
      {children}
    </div>
  );
}
