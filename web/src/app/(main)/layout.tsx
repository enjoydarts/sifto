import Nav from "@/components/nav";
import { ScrollToTop } from "@/components/scroll-to-top";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      <Nav />
      <main className="mx-auto max-w-6xl px-4 py-6 pb-[calc(env(safe-area-inset-bottom)+7.5rem)] md:pb-6">{children}</main>
      <ScrollToTop />
    </>
  );
}
