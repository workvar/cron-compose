import assert from "node:assert/strict";
import {
  bufferToBase64url,
  base64urlToBuffer,
  toCreationOptions,
  toRequestOptions,
  credentialToJSON,
  normalizePasskeyName,
  supportsConditionalMediation,
} from "./webauthn.ts";

const buf = new Uint8Array([1, 2, 255]);
assert.deepEqual(new Uint8Array(base64urlToBuffer(bufferToBase64url(buf))), buf);

{
  const encoded = bufferToBase64url(buf);
  assert.equal(encoded.includes("+"), false);
  assert.equal(encoded.includes("/"), false);
  assert.equal(encoded.endsWith("="), false);
}

{
  const challenge = bufferToBase64url(new Uint8Array([9, 8, 7]));
  const userId = bufferToBase64url(new Uint8Array([10, 11, 12]));
  const excluded = bufferToBase64url(new Uint8Array([13, 14]));
  const opts = toCreationOptions({
    challenge,
    rp: { name: "CronCompose", id: "example.test" },
    user: { id: userId, name: "ada@example.test", displayName: "Ada" },
    pubKeyCredParams: [{ type: "public-key", alg: -7 }],
    excludeCredentials: [{ type: "public-key", id: excluded }],
  });
  assert.ok(opts.publicKey);
  assert.deepEqual(new Uint8Array(opts.publicKey.challenge as ArrayBuffer), new Uint8Array([9, 8, 7]));
  assert.deepEqual(new Uint8Array(opts.publicKey.user.id as ArrayBuffer), new Uint8Array([10, 11, 12]));
  assert.deepEqual(
    new Uint8Array(opts.publicKey.excludeCredentials![0].id as ArrayBuffer),
    new Uint8Array([13, 14]),
  );
}

{
  const challenge = bufferToBase64url(new Uint8Array([1, 2, 3]));
  const allowed = bufferToBase64url(new Uint8Array([4, 5, 6]));
  const opts = toRequestOptions({
    challenge,
    rpId: "example.test",
    allowCredentials: [{ type: "public-key", id: allowed }],
  });
  assert.ok(opts.publicKey);
  assert.deepEqual(new Uint8Array(opts.publicKey.challenge as ArrayBuffer), new Uint8Array([1, 2, 3]));
  assert.deepEqual(
    new Uint8Array(opts.publicKey.allowCredentials![0].id as ArrayBuffer),
    new Uint8Array([4, 5, 6]),
  );
}

{
  const rawId = new Uint8Array([1, 2, 255]).buffer;
  const clientDataJSON = new Uint8Array([7, 7]).buffer;
  const attestationObject = new Uint8Array([8, 9]).buffer;
  const json = credentialToJSON({
    id: bufferToBase64url(rawId),
    rawId,
    type: "public-key",
    response: { clientDataJSON, attestationObject },
    getClientExtensionResults: () => ({}),
  } as unknown as PublicKeyCredential);
  const body = json as {
    rawId: string;
    response: { clientDataJSON: string; attestationObject: string };
  };
  assert.equal(body.rawId, bufferToBase64url(rawId));
  assert.equal(body.response.clientDataJSON, bufferToBase64url(clientDataJSON));
  assert.equal(body.response.attestationObject, bufferToBase64url(attestationObject));
}

{
  const rawId = new Uint8Array([9]).buffer;
  const json = credentialToJSON({
    id: bufferToBase64url(rawId),
    rawId,
    type: "public-key",
    response: {
      clientDataJSON: new Uint8Array([1]).buffer,
      authenticatorData: new Uint8Array([2]).buffer,
      signature: new Uint8Array([3]).buffer,
      userHandle: new Uint8Array([4]).buffer,
    },
    getClientExtensionResults: () => ({}),
  } as unknown as PublicKeyCredential);
  const body = json as {
    response: { authenticatorData: string; signature: string; userHandle: string };
  };
  assert.equal(body.response.authenticatorData, bufferToBase64url(new Uint8Array([2])));
  assert.equal(body.response.signature, bufferToBase64url(new Uint8Array([3])));
  assert.equal(body.response.userHandle, bufferToBase64url(new Uint8Array([4])));
}

{
  assert.equal(normalizePasskeyName("Laptop"), "Laptop");
  assert.equal(normalizePasskeyName("  YubiKey  "), "YubiKey");
  assert.equal(normalizePasskeyName(""), "Passkey");
  assert.equal(normalizePasskeyName("   "), "Passkey");
  assert.equal(normalizePasskeyName(null), "Passkey");
  assert.equal(normalizePasskeyName(undefined), "Passkey");
}

{
  const challenge = bufferToBase64url(new Uint8Array([1]));
  const opts = toRequestOptions({ challenge }, { mediation: "conditional" });
  assert.equal(opts.mediation, "conditional");
  const modal = toRequestOptions({ challenge });
  assert.equal(modal.mediation, undefined);
}

{
  const g = globalThis as { PublicKeyCredential?: unknown };
  const saved = g.PublicKeyCredential;
  g.PublicKeyCredential = undefined;
  supportsConditionalMediation().then((available) => {
    assert.equal(available, false);
    g.PublicKeyCredential = { isConditionalMediationAvailable: async () => true };
    return supportsConditionalMediation();
  }).then((available) => {
    assert.equal(available, true);
    g.PublicKeyCredential = {
      isConditionalMediationAvailable: async () => { throw new Error("unsupported"); },
    };
    return supportsConditionalMediation();
  }).then((available) => {
    assert.equal(available, false);
    g.PublicKeyCredential = saved;
  }).catch((err: unknown) => {
    g.PublicKeyCredential = saved;
    throw err;
  });
}

