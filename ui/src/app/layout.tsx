import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { Sidebar } from "@/components/sidebar";
import { ConfigProvider } from "@/components/config-provider";
import { getAllowWrites } from "@/lib/orchestrator";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Trellis",
  description: "Trellis cluster dashboard",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const allowWrites = getAllowWrites();
  const namespace = process.env.TRELLIS_NAMESPACE || "";
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="flex h-full bg-background text-foreground">
        <ConfigProvider allowWrites={allowWrites} namespace={namespace}>
          <Sidebar
            namespace={namespace || "unscoped"}
            allowWrites={allowWrites}
          />
          <main className="flex-1 overflow-y-auto">
            <div className="mx-auto max-w-5xl px-6 py-8">{children}</div>
          </main>
        </ConfigProvider>
      </body>
    </html>
  );
}
