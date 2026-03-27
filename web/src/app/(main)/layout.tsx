import Nav from "@/components/nav";
import { SharedAudioMiniPlayer } from "@/components/shared-audio-player/mini-player";
import { SharedAudioOverlay } from "@/components/shared-audio-player/overlay";
import { SharedAudioPlayerProvider } from "@/components/shared-audio-player/provider";
import { ScrollToTop } from "@/components/scroll-to-top";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <SharedAudioPlayerProvider>
      <Nav />
      <main className="mx-auto max-w-[1360px] px-4 py-6 pb-[calc(env(safe-area-inset-bottom)+12rem)] md:px-6 md:pb-6">{children}</main>
      <SharedAudioMiniPlayer />
      <SharedAudioOverlay />
      <ScrollToTop />
    </SharedAudioPlayerProvider>
  );
}
