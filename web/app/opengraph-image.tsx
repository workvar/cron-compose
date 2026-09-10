import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { ImageResponse } from "next/og";

export const alt = "CronCompose";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default async function OpenGraphImage() {
  const logo = await readFile(join(process.cwd(), "public/logo.png"));
  const src = `data:image/png;base64,${logo.toString("base64")}`;

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 40,
          background: "#e9ebe7",
        }}
      >
        <img src={src} width={168} height={168} style={{ borderRadius: 48 }} />
        <div style={{ display: "flex", flexDirection: "column" }}>
          <div
            style={{
              fontSize: 72,
              fontWeight: 800,
              color: "#14181a",
              letterSpacing: "-0.03em",
            }}
          >
            CronCompose
          </div>
          <div style={{ fontSize: 28, color: "#3c4248", marginTop: 10 }}>
            Jobs across your servers
          </div>
        </div>
      </div>
    ),
    { ...size },
  );
}
