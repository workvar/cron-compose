import type { ReactNode } from "react";

export function TerminalFrame({
  title,
  actions,
  children,
}: {
  title: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="term-wrap">
      <div className="term-bar">
        <div className="term-dots" aria-hidden="true">
          <span /><span /><span />
        </div>
        <div className="term-title">{title}</div>
        {actions}
      </div>
      {children}
    </div>
  );
}
