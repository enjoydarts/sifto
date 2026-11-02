import * as Sentry from "@sentry/nextjs";
import { sentryDataCollection } from "./src/lib/sentry-data-collection";

Sentry.init({
  dataCollection: sentryDataCollection,
  dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  environment: process.env.SENTRY_ENVIRONMENT,
  release: process.env.APP_COMMIT_SHA,
  tracesSampleRate: Number(process.env.SENTRY_TRACES_SAMPLE_RATE ?? "0"),
});
