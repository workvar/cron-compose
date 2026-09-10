import Link from "next/link";
import { LogoMark } from "./LogoMark";

type BrandProps = {
  /** When set, the wordmark is a link. Login uses no href. */
  href?: string;
};

export function Brand({ href }: BrandProps) {
  const inner = (
    <>
      <span className="mark">
        <LogoMark />
      </span>
      <span>CronCompose</span>
    </>
  );

  if (href) {
    return (
      <Link href={href} className="brand" aria-label="CronCompose home">
        {inner}
      </Link>
    );
  }

  return <div className="brand">{inner}</div>;
}
