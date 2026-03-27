type Props = {
  persona: string;
  className?: string;
};

function Blush({ left, right, fill }: { left: string; right: string; fill: string }) {
  return (
    <>
      <ellipse cx="22" cy="41" rx="4.5" ry="2.5" fill={left} opacity=".55" />
      <ellipse cx="42" cy="41" rx="4.5" ry="2.5" fill={right} opacity=".55" />
      <path d="M20 41h4" stroke={fill} strokeWidth="1.5" strokeLinecap="round" opacity=".35" />
      <path d="M40 41h4" stroke={fill} strokeWidth="1.5" strokeLinecap="round" opacity=".35" />
    </>
  );
}

export function AINavigatorAvatar({ persona, className }: Props) {
  const common = className ?? "size-11";
  switch (persona) {
    case "hype":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#D96C28" />
          <path d="M15 30c3-10 10-15 18-15 7 0 13 3 18 11l-5 1-3-5-4 4-5-3-4 5-4-4-4 2c-4 1-6 3-7 4Z" fill="#FFF4D6" />
          <path d="M19 23c4-4 8-6 13-6 6 0 10 2 14 8-4-2-9-4-14-4-5 0-9 1-13 2Z" fill="#D96C28" />
          <ellipse cx="24" cy="35" rx="3.8" ry="3.2" fill="#FFF4D6" />
          <ellipse cx="40" cy="34" rx="4.1" ry="3.4" fill="#FFF4D6" />
          <Blush left="#F7C3A2" right="#F7C3A2" fill="#FFF4D6" />
          <path d="M22 46c3 4 6 5 10 5s7-1 10-5" stroke="#FFF4D6" strokeWidth="4" strokeLinecap="round" />
          <path d="M46 14l2 6 6 2-6 2-2 6-2-6-6-2 6-2 2-6Z" fill="#FFF4D6" />
        </svg>
      );
    case "analyst":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#365F93" />
          <path d="M15 28c2-9 9-15 17-15 8 0 14 4 18 12l-2 4c-4-4-9-6-15-6-4 0-8 1-12 5h-6Z" fill="#ECF4FF" />
          <path d="M18 24c2-6 7-10 14-10 6 0 12 4 15 11-4-2-9-4-15-4-5 0-10 1-14 3Z" fill="#365F93" />
          <path d="M20 33c3-1 6-1 9 0" stroke="#ECF4FF" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M35 34c3-1 6-1 9 0" stroke="#ECF4FF" strokeWidth="3.5" strokeLinecap="round" />
          <Blush left="#ABC4E8" right="#ABC4E8" fill="#ECF4FF" />
          <path d="M23 45c2 2 5 3 9 3 4 0 7-1 9-3" stroke="#ECF4FF" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M48 40l3 3 6-9" stroke="#ECF4FF" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case "concierge":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#3C8A78" />
          <path d="M17 29c3-9 9-14 17-14 7 0 13 3 17 10l-3 3c-4-3-9-5-14-5-5 0-9 2-12 6h-5Z" fill="#EAF9F4" />
          <path d="M20 24c3-5 7-7 12-7s10 2 14 7c-4-1-9-2-14-2s-8 1-12 2Z" fill="#3C8A78" />
          <ellipse cx="24" cy="35" rx="3.4" ry="2.6" fill="#EAF9F4" />
          <ellipse cx="40" cy="35" rx="3.4" ry="2.6" fill="#EAF9F4" />
          <Blush left="#B9E5D8" right="#B9E5D8" fill="#EAF9F4" />
          <path d="M24 44c2 3 5 4 8 4s6-1 8-4" stroke="#EAF9F4" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M49 18a6 6 0 1 1-12 0 6 6 0 0 1 12 0Z" fill="#EAF9F4" opacity=".92" />
          <path d="M41 18h4m-2-2v4" stroke="#3C8A78" strokeWidth="2.5" strokeLinecap="round" />
        </svg>
      );
    case "snark":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#6E466B" />
          <path d="M16 29c3-10 10-15 19-15 8 0 14 4 18 11l-5 2c-3-4-7-6-13-6-5 0-9 3-11 8h-8Z" fill="#F9EAF8" />
          <path d="M20 24c2-5 6-8 12-8 7 0 11 3 15 9-4-2-9-3-15-3-5 0-9 1-12 2Z" fill="#6E466B" />
          <path d="M21 35c2-1 4-2 7-2" stroke="#F9EAF8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M36 33c3 0 5 1 7 2" stroke="#F9EAF8" strokeWidth="3.5" strokeLinecap="round" />
          <Blush left="#D4B3CF" right="#D4B3CF" fill="#F9EAF8" />
          <path d="M24 46c3 1 6 1 9 0 2 0 5-1 7-3" stroke="#F9EAF8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M46 18c3 0 5 2 5 5 0 4-3 6-8 9 2-4 3-7 3-9 0-2 0-4-1-5h1Z" fill="#F9EAF8" />
        </svg>
      );
    case "native":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#D24F7A" />
          <path d="M17 29c2-9 9-14 17-14 8 0 14 4 18 10l-3 3c-4-4-9-5-15-5s-9 1-12 5h-5Z" fill="#FFF0F6" />
          <path d="M20 22c3-5 7-7 12-7 6 0 11 3 14 8-4-2-8-3-14-3-4 0-8 1-12 2Z" fill="#D24F7A" />
          <ellipse cx="24" cy="35" rx="3.8" ry="3.5" fill="#FFF0F6" />
          <ellipse cx="40" cy="35.5" rx="3.4" ry="3.2" fill="#FFF0F6" />
          <Blush left="#F6BDD0" right="#F6BDD0" fill="#FFF0F6" />
          <path d="M24 44c2 3 5 4 8 4 4 0 7-1 9-4" stroke="#FFF0F6" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M47 16a6 6 0 1 1-12 0 6 6 0 0 1 12 0Z" fill="#FFF0F6" opacity=".9" />
          <path d="M38 16h6M41 13v6" stroke="#D24F7A" strokeWidth="2.5" strokeLinecap="round" />
        </svg>
      );
    case "junior":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#D85A5A" />
          <path d="M17 29c3-9 9-14 17-14 8 0 14 4 17 10l-3 3c-4-3-9-5-14-5-5 0-9 2-12 6h-5Z" fill="#FFF3FA" />
          <path d="M20 23c3-5 7-7 12-7 6 0 10 3 14 8-4-1-8-2-13-2-5 0-9 1-13 1Z" fill="#D85A5A" />
          <ellipse cx="24" cy="35" rx="3.5" ry="3.1" fill="#FFF4F1" />
          <ellipse cx="40" cy="35" rx="3.5" ry="3.1" fill="#FFF4F1" />
          <Blush left="#F2B8B3" right="#F2B8B3" fill="#FFF4F1" />
          <path d="M24 44c2 3 5 4 8 4 4 0 7-1 9-4" stroke="#FFF3FA" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M45 17c2 0 4 2 4 4s-2 4-4 4-4-2-4-4 2-4 4-4Z" fill="#FFF4F1" opacity=".95" />
          <path d="M45 15v12M39 21h12" stroke="#D85A5A" strokeWidth="2.2" strokeLinecap="round" />
        </svg>
      );
    case "urban":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#57A9D8" />
          <path d="M16 29c3-10 10-15 18-15 8 0 14 4 18 11l-3 3c-4-4-8-5-14-5-5 0-9 2-12 6h-7Z" fill="#F2FBFF" />
          <path d="M20 23c3-5 7-8 13-8s11 3 14 8c-4-1-8-2-14-2-5 0-9 1-13 2Z" fill="#57A9D8" />
          <path d="M21 35c2-1 5-1 8-1" stroke="#F2FBFF" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M35 35c3 0 6 0 8 1" stroke="#F2FBFF" strokeWidth="3.5" strokeLinecap="round" />
          <Blush left="#BFE5F6" right="#BFE5F6" fill="#F2FBFF" />
          <path d="M24 45c3 2 5 2 8 2 4 0 7-1 9-3" stroke="#F2FBFF" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M45 15h7v14h-7z" fill="#F2FBFF" opacity=".95" />
          <path d="M47 18h3M47 22h3M47 26h3" stroke="#57A9D8" strokeWidth="2.2" strokeLinecap="round" />
        </svg>
      );
    default:
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#7A4A2C" />
          <path d="M15 28c3-10 10-15 18-15 9 0 15 4 19 12l-4 2c-4-4-8-6-14-6-5 0-9 3-11 8h-8Z" fill="#FFF2E8" />
          <path d="M18 23c3-6 8-9 14-9 7 0 12 3 15 10-4-2-9-3-15-3-5 0-10 1-14 2Z" fill="#7A4A2C" />
          <ellipse cx="24" cy="35" rx="3.3" ry="3" fill="#FFF2E8" />
          <ellipse cx="40" cy="35" rx="3.3" ry="3" fill="#FFF2E8" />
          <Blush left="#EAC6B1" right="#EAC6B1" fill="#FFF2E8" />
          <path d="M24 45c2 2 5 3 8 3s6-1 8-3" stroke="#FFF2E8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M45 16h8v10h-8z" fill="#FFF2E8" />
          <path d="M43 20h12" stroke="#7A4A2C" strokeWidth="2.5" strokeLinecap="round" />
          <path d="M43 24h12" stroke="#7A4A2C" strokeWidth="2.5" strokeLinecap="round" />
        </svg>
      );
  }
}
