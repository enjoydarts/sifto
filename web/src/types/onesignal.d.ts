export {};

interface OneSignalPushSubscription {
  optedIn?: boolean;
  optIn?: () => Promise<void>;
  optOut?: () => Promise<void>;
}

interface OneSignalLike {
  init: (options: Record<string, unknown>) => Promise<void>;
  login?: (externalId: string) => Promise<void>;
  Notifications?: {
    requestPermission?: () => Promise<void>;
  };
  User?: {
    PushSubscription?: OneSignalPushSubscription;
  };
}

type OneSignalLegacyQueueItem = () => void | Promise<void>;

declare global {
  interface Window {
    OneSignalDeferred?: Array<(OneSignal: OneSignalLike) => void | Promise<void>>;
    OneSignal?: OneSignalLike | OneSignalLegacyQueueItem[];
    __siftoGetAuthToken?: () => Promise<string | null>;
    __siftoClerkIdentityResolved?: boolean;
    __siftoClerkIdentityKey?: string;
    __siftoClerkIdentityPromise?: Promise<void>;
    __siftoOneSignalLoading?: boolean;
    __siftoOneSignalReady?: boolean;
    __siftoOneSignalInitError?: string;
    __siftoOneSignalScriptLoaded?: boolean;
    __siftoOneSignalScriptError?: string;
    __siftoOneSignalScriptRequestedAt?: number;
    __siftoOneSignalLoadAttempt?: number;
    __siftoOneSignalDeferredExecuted?: boolean;
    __siftoOneSignalInitEnqueued?: number;
  }
}
