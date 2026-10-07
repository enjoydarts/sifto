type EmailUser = {
  primaryEmailAddressId: string | null;
  emailAddresses: { id: string; emailAddress: string; verification?: { status: string } | null }[];
};

export function verifiedPrimaryEmail(user: EmailUser | null | undefined): string | null {
  const primary = user?.emailAddresses.find((entry) => entry.id === user.primaryEmailAddressId);
  return primary?.verification?.status === "verified" ? primary.emailAddress : null;
}
