import type { Metadata, Viewport } from "next";
import { Sawarabi_Gothic, Sawarabi_Mincho } from "next/font/google";
import Script from "next/script";
import "./globals.css";
import { Providers } from "@/components/providers";

const sawarabiGothic = Sawarabi_Gothic({
  subsets: ["latin"],
  variable: "--font-sans-jp",
  weight: ["400"],
});

const sawarabiMincho = Sawarabi_Mincho({
  subsets: ["latin"],
  variable: "--font-serif-jp",
  weight: ["400"],
});

export const metadata: Metadata = {
  title: "Sifto",
  description: "RSS aggregator & daily digest",
  manifest: "/manifest.webmanifest",
  icons: {
    apple: "/apple-touch-icon.png",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const clerkPublishableKey = process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY;
  const clerkEnabled = Boolean(clerkPublishableKey && process.env.CLERK_SECRET_KEY);

  return (
    <html lang="ja" className={`${sawarabiGothic.variable} ${sawarabiMincho.variable}`}>
      <body className="min-h-screen bg-zinc-50 font-sans text-zinc-900 antialiased">
        <Script
          id="onesignal-sdk"
          src="https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js"
          strategy="lazyOnload"
        />
        <Providers clerkEnabled={clerkEnabled} clerkPublishableKey={clerkPublishableKey}>
          {children}
        </Providers>
      </body>
    </html>
  );
}
