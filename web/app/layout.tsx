import "./globals.css";
import "./components.css";
import "./layout.css";
import "./dashboard.css";
import "./wizard.css";
import "./terminal.css";
import "./connectors.css";
import "./landing.css";
import type { Metadata } from "next";
import { Outfit, Source_Sans_3 } from "next/font/google";
import { Shell } from "@/components/Shell";
import { ThemeProvider, themeBootScript } from "@/components/ThemeProvider";
import { UpdatingOverlay } from "@/components/UpdatingOverlay";

const outfit = Outfit({
  subsets: ["latin"],
  variable: "--font-display",
  display: "swap",
});

const sourceSans = Source_Sans_3({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

export const metadata: Metadata = {
  title: "CronCompose",
  description: "Deploy apps and schedule jobs across remote Linux servers",
  icons: {
    icon: [
      { url: "/app/logo.svg", type: "image/svg+xml" },
      { url: "/app/logo.png", type: "image/png", sizes: "512x512" },
    ],
    apple: [{ url: "/app/apple-touch-icon.png", sizes: "180x180", type: "image/png" }],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${outfit.variable} ${sourceSans.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeBootScript }} />
      </head>
      <body>
        <ThemeProvider>
          <Shell>{children}</Shell>
          <UpdatingOverlay />
        </ThemeProvider>
      </body>
    </html>
  );
}
