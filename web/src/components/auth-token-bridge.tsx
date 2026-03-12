"use client";

import { useEffect } from "react";
import { useAuth, useUser } from "@clerk/nextjs";

async function resolveClerkIdentity() {
  const res = await fetch("/api/auth/clerk/resolve-identity", {
    method: "POST",
    cache: "no-store",
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `resolve identity failed: ${res.status}`);
  }
}

export default function AuthTokenBridge() {
  const { isLoaded, isSignedIn, getToken, userId } = useAuth();
  const { user } = useUser();

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.__siftoGetAuthToken = async () => {
      if (!isLoaded || !isSignedIn) return null;
      return getToken();
    };
    return () => {
      window.__siftoGetAuthToken = undefined;
    };
  }, [getToken, isLoaded, isSignedIn]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!isLoaded) return;
    if (!isSignedIn || !userId) {
      window.__siftoClerkIdentityResolved = false;
      window.__siftoClerkIdentityKey = undefined;
      return;
    }

    const identityKey = `${userId}:${user?.primaryEmailAddress?.emailAddress ?? ""}`;
    if (window.__siftoClerkIdentityResolved && window.__siftoClerkIdentityKey === identityKey) {
      return;
    }
    if (window.__siftoClerkIdentityPromise) {
      return;
    }

    const promise = resolveClerkIdentity()
      .then(() => {
        window.__siftoClerkIdentityResolved = true;
        window.__siftoClerkIdentityKey = identityKey;
      })
      .catch(() => {
        window.__siftoClerkIdentityResolved = false;
      })
      .finally(() => {
        window.__siftoClerkIdentityPromise = undefined;
      });

    window.__siftoClerkIdentityPromise = promise;
  }, [isLoaded, isSignedIn, user?.primaryEmailAddress?.emailAddress, userId]);

  return null;
}
