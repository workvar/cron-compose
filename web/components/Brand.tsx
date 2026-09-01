import Link from "next/link";
import logo from "../public/logo.png";

type BrandProps = {
  /** When set, the wordmark is a link. Login uses no href. */
  href?: string;
};

export function Brand({ href }: BrandProps) {
  const inner = (
    <>
      <span className="mark">
        <img src={logo.src} alt="" width={34} height={34} />
      </span>
      <span>CronCompose</span>
    </>
  );

  if (href) {
    return (
      <Link href={href} className="brand" aria-label="CronCompose">
        {inner}
      </Link>
    );
  }

  return <div className="brand">{inner}</div>;
}
