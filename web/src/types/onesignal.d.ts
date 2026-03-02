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

declare global {
  interface Window {
    OneSignalDeferred?: Array<(OneSignal: OneSignalLike) => void | Promise<void>>;
    OneSignal?: OneSignalLike;
    __siftoOneSignalLoading?: boolean;
    __siftoOneSignalReady?: boolean;
  }
}

