import { LogoMark } from "./LogoMark";

type BrandLoaderProps = {
  /** Accessible label. */
  label?: string;
  /** Mark size in px. */
  size?: number;
  /** Optional caption under the mark. */
  caption?: string;
  className?: string;
  /** Compact inline variant (no caption stacking). */
  inline?: boolean;
};

/** Logo-inspired loading mark used across the app. */
export function BrandLoader({
  label = "Loading",
  size = 40,
  caption,
  className = "",
  inline = false,
}: BrandLoaderProps) {
  return (
    <div
      className={`brand-loader${inline ? " is-inline" : ""}${className ? ` ${className}` : ""}`}
      role="status"
      aria-live="polite"
      aria-label={label}
    >
      <span className="brand-loader-mark" style={{ width: size, height: size }}>
        <LogoMark size={size} className="brand-loader-svg" />
        <span className="brand-loader-orbit" aria-hidden="true" />
      </span>
      {caption ? <span className="brand-loader-caption">{caption}</span> : null}
      <span className="sr-only">{label}</span>
    </div>
  );
}

/** Centered page/section placeholder. */
export function BrandLoaderBlock({
  caption = "Loading…",
  size = 48,
}: {
  caption?: string;
  size?: number;
}) {
  return (
    <div className="brand-loader-block">
      <BrandLoader size={size} caption={caption} label={caption} />
    </div>
  );
}
