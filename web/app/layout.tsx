import "./globals.css";
import "./components.css";
import "./layout.css";
import "./dashboard.css";
import "./wizard.css";
import "./terminal.css";
import "./connectors.css";
import type { Metadata } from "next";
import { Shell } from "@/components/Shell";

export const metadata: Metadata = {
  title: "CronCompose",
  description: "Schedule and manage jobs across remote Linux servers",
  icons: {
    icon: [{ url: "/app/logo.png", type: "image/png" }],
    apple: [{ url: "/app/logo.png", type: "image/png" }],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
