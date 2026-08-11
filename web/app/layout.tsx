import "./globals.css";
import "./components.css";
import "./layout.css";
import "./dashboard.css";
import "./wizard.css";
import "./terminal.css";
import type { Metadata } from "next";
import { Shell } from "@/components/Shell";

export const metadata: Metadata = {
  title: "CronCompose",
  description: "Schedule and manage jobs across remote Linux servers",
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
