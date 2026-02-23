import Nav from "@/components/nav";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      <Nav />
      <main className="mx-auto max-w-4xl px-4 py-8">{children}</main>
    </>
  );
}
