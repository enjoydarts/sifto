type Props = {
  persona: string;
  className?: string;
};

export function AINavigatorAvatar({ persona, className }: Props) {
  const common = className ?? "size-11";
  switch (persona) {
    case "hype":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#D96C28" />
          <path d="M18 29c4-10 11-15 20-15 7 0 12 3 16 10l-7 1c-2-3-5-5-9-5-5 0-9 3-12 9h-8Z" fill="#FFF4D6" />
          <circle cx="24" cy="34" r="3" fill="#FFF4D6" />
          <circle cx="40" cy="34" r="3" fill="#FFF4D6" />
          <path d="M23 46c3 3 6 4 9 4s6-1 9-4" stroke="#FFF4D6" strokeWidth="4" strokeLinecap="round" />
          <path d="M46 14l2 6 6 2-6 2-2 6-2-6-6-2 6-2 2-6Z" fill="#FFF4D6" />
        </svg>
      );
    case "analyst":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#365F93" />
          <path d="M15 28c2-9 9-15 17-15 8 0 14 4 18 12l-6 2c-2-5-6-7-11-7s-9 3-11 8h-7Z" fill="#ECF4FF" />
          <path d="M19 34h11" stroke="#ECF4FF" strokeWidth="4" strokeLinecap="round" />
          <path d="M34 34h11" stroke="#ECF4FF" strokeWidth="4" strokeLinecap="round" />
          <path d="M23 45c2 2 5 3 9 3 4 0 7-1 9-3" stroke="#ECF4FF" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M48 40l3 3 6-9" stroke="#ECF4FF" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case "concierge":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#3C8A78" />
          <path d="M17 29c3-9 9-14 17-14 7 0 13 3 17 10l-7 2c-2-3-6-5-10-5-5 0-9 2-11 7h-6Z" fill="#EAF9F4" />
          <circle cx="25" cy="35" r="2.8" fill="#EAF9F4" />
          <circle cx="39" cy="35" r="2.8" fill="#EAF9F4" />
          <path d="M24 44c2 3 5 4 8 4s6-1 8-4" stroke="#EAF9F4" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M50 18a6 6 0 1 1-12 0 6 6 0 0 1 12 0Z" fill="#EAF9F4" opacity=".9" />
          <path d="M42 18h4m-2-2v4" stroke="#3C8A78" strokeWidth="2.5" strokeLinecap="round" />
        </svg>
      );
    case "snark":
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#6E466B" />
          <path d="M16 29c3-10 10-15 19-15 8 0 14 4 18 11l-7 1c-2-3-6-5-11-5-5 0-9 3-11 8h-8Z" fill="#F9EAF8" />
          <path d="M21 35c2-1 4-2 7-2" stroke="#F9EAF8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M36 33c3 0 5 1 7 2" stroke="#F9EAF8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M24 46c3 1 6 1 9 0 2 0 5-1 7-3" stroke="#F9EAF8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M46 18c3 0 5 2 5 5 0 4-3 6-8 9 2-4 3-7 3-9 0-2 0-4-1-5h1Z" fill="#F9EAF8" />
        </svg>
      );
    default:
      return (
        <svg viewBox="0 0 64 64" className={common} aria-hidden="true" fill="none">
          <circle cx="32" cy="32" r="30" fill="#7A4A2C" />
          <path d="M15 28c3-10 10-15 18-15 9 0 15 4 19 12l-7 1c-3-4-7-6-12-6-5 0-9 3-11 8h-7Z" fill="#FFF2E8" />
          <circle cx="24" cy="35" r="3" fill="#FFF2E8" />
          <circle cx="40" cy="35" r="3" fill="#FFF2E8" />
          <path d="M24 45c2 2 5 3 8 3s6-1 8-3" stroke="#FFF2E8" strokeWidth="3.5" strokeLinecap="round" />
          <path d="M45 16h8v10h-8z" fill="#FFF2E8" />
          <path d="M43 20h12" stroke="#7A4A2C" strokeWidth="2.5" strokeLinecap="round" />
          <path d="M43 24h12" stroke="#7A4A2C" strokeWidth="2.5" strokeLinecap="round" />
        </svg>
      );
  }
}
