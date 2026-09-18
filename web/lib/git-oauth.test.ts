import assert from "node:assert/strict";
import {
  defaultOAuthCallbackUrl,
  gitOAuthConnectAction,
  gitOAuthStartFetchAction,
  gitOAuthStartUrl,
  oauthAppSaveReady,
} from "./git-oauth.ts";

assert.equal(
  gitOAuthStartUrl("https://cron.example.com", "github", "/app/settings"),
  "https://cron.example.com/api/v1/auth/github/start?purpose=git&next=%2Fapp%2Fsettings",
);
assert.equal(
  gitOAuthStartUrl("https://cron.example.com", "gitlab", "/app/deploys/new"),
  "https://cron.example.com/api/v1/auth/gitlab/start?purpose=git&next=%2Fapp%2Fdeploys%2Fnew",
);

assert.equal(gitOAuthConnectAction({ configLoaded: false, isAdmin: true, enabled: false }), "pending");
assert.equal(gitOAuthConnectAction({ configLoaded: true, isAdmin: true, enabled: false }), "setup");
assert.equal(gitOAuthConnectAction({ configLoaded: true, isAdmin: false, enabled: false }), "ask-admin");
assert.equal(gitOAuthConnectAction({ configLoaded: true, isAdmin: true, enabled: true }), "start");
assert.equal(gitOAuthConnectAction({ configLoaded: true, isAdmin: true }), "start");

assert.equal(oauthAppSaveReady("", "secret", false), false);
assert.equal(oauthAppSaveReady("id", "", false), false);
assert.equal(oauthAppSaveReady("id", "secret", false), true);
assert.equal(oauthAppSaveReady("id", "", true), true);

assert.equal(
  defaultOAuthCallbackUrl("https://cron.example.com", "github"),
  "https://cron.example.com/api/auth/github/callback",
);
assert.equal(
  defaultOAuthCallbackUrl("https://cron.example.com/", "gitlab"),
  "https://cron.example.com/api/auth/gitlab/callback",
);

// A 302 to the provider (or the opaque redirect fetch() turns it into) means
// the OAuth app is configured. Treating that as "not configured" reopens the
// setup modal after a successful save.
assert.equal(
  gitOAuthStartFetchAction({ ok: false, status: 0, type: "opaqueredirect", isAdmin: true }),
  "navigate",
);
assert.equal(
  gitOAuthStartFetchAction({ ok: false, status: 302, isAdmin: true }),
  "navigate",
);
assert.equal(
  gitOAuthStartFetchAction({
    ok: false,
    status: 404,
    errorMessage: "github oauth is not configured",
    isAdmin: true,
  }),
  "setup",
);
assert.equal(
  gitOAuthStartFetchAction({
    ok: false,
    status: 404,
    errorMessage: "github oauth is not configured",
    isAdmin: false,
  }),
  "ask-admin",
);
