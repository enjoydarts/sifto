import type { Metadata, Viewport } from "next";
import Script from "next/script";
import { ClerkProvider } from "@clerk/nextjs";
import { headers } from "next/headers";
import "./globals.css";
import { Providers } from "@/components/providers";
import { itemsSSREnabled, itemsSSRRequested } from "@/lib/items-ssr-request";

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
  maximumScale: 1,
  userScalable: false,
  viewportFit: "cover",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const clerkPublishableKey = process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY;
  const clerkEnabled = Boolean(clerkPublishableKey);
  const itemsSSR = clerkEnabled && itemsSSREnabled() && itemsSSRRequested(await headers());
  const content = (
    <Providers clerkEnabled={clerkEnabled} clerkPublishableKey={clerkPublishableKey} clerkContextProvided={itemsSSR}>
      {children}
    </Providers>
  );

  return (
    <html lang="ja">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          rel="stylesheet"
          href="https://fonts.googleapis.com/css2?family=Sawarabi+Gothic&family=Sawarabi+Mincho&display=swap"
        />
      </head>
      <body
        className="min-h-screen bg-[var(--background)] font-sans text-[var(--foreground)] antialiased"
      >
        <Script
          id="onesignal-sdk"
          src="https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js"
          strategy="lazyOnload"
        />
        {itemsSSR ? <ClerkProvider dynamic publishableKey={clerkPublishableKey}>{content}</ClerkProvider> : content}
      </body>
    </html>
  );
}
