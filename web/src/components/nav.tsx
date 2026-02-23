"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession, signOut } from "next-auth/react";

const links = [
  { href: "/items", label: "Items" },
  { href: "/sources", label: "Sources" },
  { href: "/digests", label: "Digests" },
  { href: "/debug/digests", label: "Debug" },
];

export default function Nav() {
  const pathname = usePathname();
  const { data: session } = useSession();

  return (
    <header className="border-b border-zinc-200 bg-white">
      <div className="mx-auto flex h-14 max-w-4xl items-center gap-6 px-4">
        <Link href="/" className="flex items-center gap-2">
          <Image src="/logo.png" alt="Sifto" width={28} height={28} priority />
          <span className="text-lg font-bold tracking-tight text-zinc-900">
            Sifto
          </span>
        </Link>

        <nav className="flex flex-1 gap-1">
          {links.map(({ href, label }) => (
            <Link
              key={href}
              href={href}
              className={`rounded px-3 py-1.5 text-sm font-medium transition-colors ${
                pathname === href || pathname.startsWith(`${href}/`)
                  ? "bg-zinc-100 text-zinc-900"
                  : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
              }`}
            >
              {label}
            </Link>
          ))}
        </nav>

        {session?.user && (
          <div className="flex items-center gap-3">
            <span className="text-sm text-zinc-500">
              {session.user.name ?? session.user.email}
            </span>
            <button
              onClick={() => signOut({ callbackUrl: "/login" })}
              className="rounded border border-zinc-200 px-3 py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-50 hover:text-zinc-900"
            >
              Sign out
            </button>
          </div>
        )}
      </div>
    </header>
  );
}
