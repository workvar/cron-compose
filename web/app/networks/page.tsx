"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { SearchableSelect } from "@/components/SearchableSelect";
import { IconNetwork } from "@/components/icons";
import type { ListResponse, Me, Server } from "@/lib/types";
import "./networks.css";

type DualWifi = { supported: boolean; detail?: string };
type NetIface = {
  name: string;
  type: string;
  state: string;
  connection?: string;
  ipv4?: string[];
  ipv6?: string[];
  gateway?: string;
  dns?: string[];
  method?: string;
  primary?: boolean;
};
type WifiNet = {
  ssid: string;
  connection?: string;
  signal?: number;
  security?: string;
  active?: boolean;
  device?: string;
  bssid?: string;
};
type BtDevice = {
  address: string;
  name?: string;
  paired?: boolean;
  trusted?: boolean;
  connected?: boolean;
};
type PANLink = {
  address: string;
  name?: string;
  connection?: string;
  ipv4?: string[];
  active?: boolean;
};
type Modem = {
  id: string;
  state?: string;
  operator?: string;
  signal?: number;
  apn?: string;
  connected?: boolean;
  ipv4?: string[];
  connection?: string;
};
type NetworkStatus = {
  backend: string;
  dual_wifi: DualWifi;
  interfaces: NetIface[];
  wifi: { enabled: boolean; active?: WifiNet; saved: WifiNet[]; device?: string };
  bluetooth: { available: boolean; powered?: boolean; devices: BtDevice[]; pan: PANLink[] };
  cellular: { available: boolean; modems: Modem[] };
  default_route?: string;
  control_plane_iface?: string;
  capabilities?: string[];
  error?: string;
};

type PinChallenge = {
  requestId: string;
  device: string;
  method: string;
  prompt: string;
};

type Mode = "wireless" | "wired" | "bluetooth" | "cellular";
type WifiFlowStep = "pick" | "credentials" | "review";

async function apiJSON<T = Record<string, unknown>>(
  path: string,
  init?: RequestInit,
): Promise<{ ok: boolean; status: number; body: T & { error?: string; result?: unknown } }> {
  const res = await fetch(path, {
    ...init,
    headers: { "content-type": "application/json", ...(init?.headers ?? {}) },
  });
  const body = (await res.json().catch(() => ({}))) as T & { error?: string; result?: unknown };
  return { ok: res.ok, status: res.status, body };
}

function isVirtualIface(name: string): boolean {
  return /^(veth|br-|docker|virbr|cni|flannel|cali|tun|tap)/i.test(name);
}

function Chevron() {
  return (
    <svg className="networks-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}

function Pill({ ok, children }: { ok?: boolean; children: ReactNode }) {
  return <span className={`pill ${ok ? "ok" : "neutral"}`}>{children}</span>;
}

export default function NetworksPage() {
  const [servers, setServers] = useState<Server[]>([]);
  const [serverId, setServerId] = useState("");
  const [me, setMe] = useState<Me | null>(null);
  const [status, setStatus] = useState<NetworkStatus | null>(null);
  const [scan, setScan] = useState<WifiNet[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<Mode>("wireless");

  const [openWifi, setOpenWifi] = useState<string | null>(null);
  const [openWired, setOpenWired] = useState<string | null>(null);
  const [openBt, setOpenBt] = useState<string | null>(null);
  const [showVirtual, setShowVirtual] = useState(false);

  const [wifiFlow, setWifiFlow] = useState(false);
  const [wifiStep, setWifiStep] = useState<WifiFlowStep>("pick");
  const [wifiSSID, setWifiSSID] = useState("");
  const [wifiPSK, setWifiPSK] = useState("");
  const [wifiConnectAfter, setWifiConnectAfter] = useState(false);
  const [wifiAlongside, setWifiAlongside] = useState(false);

  const [pskEdit, setPskEdit] = useState<string | null>(null);
  const [pskValue, setPskValue] = useState("");

  const [wiredMethod, setWiredMethod] = useState("dhcp");
  const [wiredAddress, setWiredAddress] = useState("");
  const [wiredPrefix, setWiredPrefix] = useState("24");
  const [wiredGateway, setWiredGateway] = useState("");
  const [wiredDNS, setWiredDNS] = useState("");

  const [cellAPN, setCellAPN] = useState("");
  const [cellConn, setCellConn] = useState("cc-cellular");

  const [pin, setPin] = useState<PinChallenge | null>(null);
  const [pinValue, setPinValue] = useState("");

  const canMutate = me?.role === "admin" || me?.role === "owner";
  const dualOk = !!status?.dual_wifi?.supported;

  useEffect(() => {
    let live = true;
    fetch("/api/me")
      .then((r) => r.json() as Promise<Me>)
      .then((d) => live && setMe(d))
      .catch(() => live && setMe(null));
    fetch("/api/servers")
      .then((r) => r.json() as Promise<ListResponse<Server>>)
      .then((d) => {
        if (!live) return;
        const items = d.items || [];
        setServers(items);
        if (!serverId && items[0]) setServerId(items[0].id);
      })
      .catch(() => live && setServers([]));
    return () => {
      live = false;
    };
  }, [serverId]);

  const serverOptions = useMemo(
    () => servers.map((s) => ({ value: s.id, label: `${s.name} · ${s.status}` })),
    [servers],
  );

  const refresh = useCallback(async () => {
    if (!serverId) return;
    setError(null);
    const { ok, body } = await apiJSON<{ result?: NetworkStatus; error?: string }>(
      `/api/servers/${serverId}/network`,
    );
    if (!ok) {
      setStatus(null);
      setError(typeof body.error === "string" ? body.error : "Could not load network status");
      return;
    }
    setStatus((body.result || body) as NetworkStatus);
  }, [serverId]);

  useEffect(() => {
    if (serverId) void refresh();
  }, [serverId, refresh]);

  async function mutate(path: string, body?: unknown) {
    if (!serverId || !canMutate) return;
    setBusy(true);
    setError(null);
    try {
      const { ok, body: res } = await apiJSON(`/api/servers/${serverId}${path}`, {
        method: "POST",
        body: JSON.stringify(body ?? {}),
      });
      if (!ok) throw new Error(res.error || "request failed");
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function doWifiScan() {
    if (!serverId) return;
    setBusy(true);
    setError(null);
    try {
      const { ok, body } = await apiJSON<{ result?: WifiNet[]; error?: string }>(
        `/api/servers/${serverId}/network/wifi/scan`,
      );
      if (!ok) throw new Error(body.error || "scan failed");
      setScan(Array.isArray(body.result) ? body.result : []);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function startWifiFlow() {
    setWifiFlow(true);
    setWifiStep("pick");
    setWifiSSID("");
    setWifiPSK("");
    setWifiConnectAfter(false);
    setWifiAlongside(false);
    setScan([]);
    void doWifiScan();
  }

  function cancelWifiFlow() {
    setWifiFlow(false);
    setWifiStep("pick");
    setWifiSSID("");
    setWifiPSK("");
  }

  async function finishWifiFlow() {
    if (!wifiSSID) return;
    setBusy(true);
    setError(null);
    try {
      const save = await apiJSON(`/api/servers/${serverId}/network/wifi/save`, {
        method: "POST",
        body: JSON.stringify({ ssid: wifiSSID, psk: wifiPSK }),
      });
      if (!save.ok) throw new Error(save.body.error || "save failed");
      if (wifiConnectAfter) {
        const up = await apiJSON(`/api/servers/${serverId}/network/wifi/connect`, {
          method: "POST",
          body: JSON.stringify({
            ssid: wifiSSID,
            psk: wifiPSK,
            alongside: wifiAlongside && dualOk,
          }),
        });
        if (!up.ok) throw new Error(up.body.error || "connect failed");
      }
      cancelWifiFlow();
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function selectWired(iface: NetIface) {
    const key = iface.name;
    if (openWired === key) {
      setOpenWired(null);
      return;
    }
    setOpenWired(key);
    setWiredMethod(iface.method === "manual" ? "static" : "dhcp");
    if (iface.ipv4?.[0]) {
      const [addr, pref] = iface.ipv4[0].split("/");
      setWiredAddress(addr || "");
      setWiredPrefix(pref || "24");
    } else {
      setWiredAddress("");
      setWiredPrefix("24");
    }
    setWiredGateway(iface.gateway || "");
    setWiredDNS((iface.dns || []).join(", "));
  }

  async function pairBluetooth(address: string) {
    if (!serverId || !canMutate) return;
    setBusy(true);
    setError(null);
    setPin(null);
    try {
      const res = await fetch(`/api/servers/${serverId}/network/bluetooth/pair`, {
        method: "POST",
        headers: { "content-type": "application/json", accept: "text/event-stream" },
        body: JSON.stringify({ address_bt: address }),
      });
      const ctype = res.headers.get("content-type") || "";
      if (!res.ok && !ctype.includes("text/event-stream")) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `HTTP ${res.status}`);
      }
      const reader = res.body?.getReader();
      if (!reader) throw new Error("no stream");
      const dec = new TextDecoder();
      let buf = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        const chunks = buf.split("\n\n");
        buf = chunks.pop() || "";
        for (const block of chunks) {
          const lines = block.split("\n");
          let event = "message";
          let data = "";
          for (const line of lines) {
            if (line.startsWith("event:")) event = line.slice(6).trim();
            if (line.startsWith("data:")) data += line.slice(5).trim();
          }
          if (!data) continue;
          const payload = JSON.parse(data) as Record<string, string>;
          if (event === "pin_required" || payload.kind === "pin_required") {
            setPin({
              requestId: payload.request_id,
              device: payload.pin_device || address,
              method: payload.pin_method || "pin",
              prompt: payload.pin_prompt || "Enter PIN",
            });
          }
          if (event === "done" && payload.status === "failed") {
            throw new Error(payload.error || "pair failed");
          }
        }
      }
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function submitPin() {
    if (!serverId || !pin) return;
    setBusy(true);
    try {
      const { ok, body } = await apiJSON(`/api/servers/${serverId}/network/bluetooth/pin`, {
        method: "POST",
        body: JSON.stringify({ request_id: pin.requestId, pin: pinValue }),
      });
      if (!ok) throw new Error(body.error || "pin rejected");
      setPin(null);
      setPinValue("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const ethernetAll = (status?.interfaces || []).filter((i) => i.type === "ethernet");
  const ethernet = showVirtual ? ethernetAll : ethernetAll.filter((i) => !isVirtualIface(i.name));
  const virtualCount = ethernetAll.length - ethernetAll.filter((i) => !isVirtualIface(i.name)).length;

  const linkSummary = useMemo(() => {
    if (!status) return null;
    if (status.wifi?.active?.ssid) {
      return {
        on: true,
        title: status.wifi.active.ssid,
        detail: `Wi‑Fi${status.wifi.active.signal != null ? ` · ${status.wifi.active.signal}%` : ""}${
          status.control_plane_iface ? ` · via ${status.control_plane_iface}` : ""
        }`,
        kind: "Wi‑Fi",
      };
    }
    const eth = ethernetAll.find((i) => i.primary) || ethernetAll.find((i) => i.state.includes("connected") && !isVirtualIface(i.name));
    if (eth) {
      return {
        on: eth.state.includes("connected"),
        title: eth.name,
        detail: `${(eth.ipv4 || [])[0] || "no IPv4"}${eth.gateway ? ` · gw ${eth.gateway}` : ""}`,
        kind: "Ethernet",
      };
    }
    const modem = status.cellular?.modems?.find((m) => m.connected);
    if (modem) {
      return {
        on: true,
        title: modem.operator || `Modem ${modem.id}`,
        detail: modem.apn ? `Cellular · ${modem.apn}` : "Cellular",
        kind: "Cellular",
      };
    }
    return {
      on: false,
      title: "No active link reported",
      detail: status.backend === "unavailable" ? "Network backend unavailable" : `Backend · ${status.backend}`,
      kind: "Offline",
    };
  }, [status, ethernetAll]);

  const modes: { id: Mode; label: string; count?: number; hide?: boolean }[] = [
    { id: "wireless", label: "Wireless", count: status?.wifi?.saved?.length },
    { id: "wired", label: "Wired", count: ethernet.length },
    {
      id: "bluetooth",
      label: "Bluetooth",
      count: status?.bluetooth?.devices?.length,
      hide: status != null && !status.bluetooth?.available,
    },
    {
      id: "cellular",
      label: "Cellular",
      count: status?.cellular?.modems?.length,
      hide: status != null && !status.cellular?.available && !(status.cellular?.modems?.length),
    },
  ];

  return (
    <div className="page networks-page">
      <div className="page-head">
        <div>
          <h1>
            <IconNetwork />
            Networks
          </h1>
          <p className="subtle" style={{ marginTop: 6 }}>
            Configure how this agent reaches the internet — one connection type at a time.
          </p>
        </div>
        {status && (
          <div className="page-head-actions">
            <span className="networks-backend">
              {status.backend}
              {status.control_plane_iface ? ` · ${status.control_plane_iface}` : ""}
            </span>
          </div>
        )}
      </div>

      <div className="panel networks-toolbar">
        <div className="field">
          <label htmlFor="net-server">Server</label>
          {servers.length === 0 ? (
            <p className="field-hint">
              No servers yet. <Link href="/servers/new">Add a server</Link> first.
            </p>
          ) : (
            <SearchableSelect
              id="net-server"
              value={serverId}
              onChange={(id) => {
                setServerId(id);
                setStatus(null);
                setScan([]);
                cancelWifiFlow();
                setOpenWifi(null);
                setOpenWired(null);
              }}
              options={serverOptions}
              placeholder="Select a server…"
              aria-label="Server"
            />
          )}
        </div>
        <div className="networks-toolbar-actions">
          {!canMutate && <span className="networks-muted">View only</span>}
          <button
            type="button"
            className="button secondary sm"
            onClick={() => void refresh()}
            disabled={!serverId || busy}
          >
            {busy ? "Working…" : "Refresh"}
          </button>
        </div>
      </div>

      {error && <div className="form-error" style={{ marginBottom: 16 }}>{error}</div>}

      {!serverId && <p className="networks-empty">Pick a server to manage its networks.</p>}
      {serverId && !status && !error && <p className="networks-empty">Loading network status…</p>}

      {status && linkSummary && (
        <div className="networks-link-card">
          <span className={`networks-link-pulse${linkSummary.on ? " on" : ""}`} aria-hidden />
          <div className="networks-link-meta">
            <strong>{linkSummary.title}</strong>
            <span>{linkSummary.detail}</span>
          </div>
          <div className="networks-link-stat">{linkSummary.kind}</div>
        </div>
      )}

      {status && (
        <>
          <div className="networks-tabs" role="tablist">
            {modes.filter((m) => !m.hide).map((m) => (
              <button
                key={m.id}
                type="button"
                role="tab"
                aria-selected={mode === m.id}
                className={`networks-tab${mode === m.id ? " active" : ""}`}
                onClick={() => {
                  setMode(m.id);
                  cancelWifiFlow();
                  setPskEdit(null);
                }}
              >
                {m.label}
                {m.count != null && m.count > 0 ? <span className="count">{m.count}</span> : null}
              </button>
            ))}
          </div>

          {mode === "wireless" && (
            <section className="networks-panel">
              <div className="networks-panel-head">
                <div>
                  <h2>Wireless</h2>
                  <p>
                    Save networks without dropping the current link. Change a password in place,
                    then connect when you are ready
                    {dualOk ? " — or join a second SSID alongside the active one." : "."}
                  </p>
                </div>
                {canMutate && !wifiFlow && (
                  <button type="button" className="button primary sm" disabled={busy} onClick={startWifiFlow}>
                    Join a network
                  </button>
                )}
              </div>

              <div className="networks-panel-body">
                {(status.wifi.saved || []).length === 0 && !wifiFlow ? (
                  <div className="networks-empty">
                    <strong>No saved Wi‑Fi profiles</strong>
                    Scan and join a network to get started.
                  </div>
                ) : (
                  <ul className="networks-list">
                    {(status.wifi.saved || []).map((w) => {
                      const key = w.connection || w.ssid;
                      const open = openWifi === key;
                      return (
                        <li key={key} className={`networks-item${open ? " open" : ""}`}>
                          <button
                            type="button"
                            className="networks-item-btn"
                            onClick={() => {
                              setOpenWifi(open ? null : key);
                              setPskEdit(null);
                            }}
                          >
                            <div className="networks-item-main">
                              <strong>{w.ssid || key}</strong>
                              <span>{w.connection || "saved profile"}</span>
                            </div>
                            <div className="networks-item-side">
                              {w.active ? <Pill ok>active</Pill> : null}
                              <Chevron />
                            </div>
                          </button>
                          {open && (
                            <div className="networks-drawer">
                              {pskEdit === key ? (
                                <>
                                  <p className="networks-drawer-note">
                                    Updates the saved password only — does not reconnect.
                                  </p>
                                  <div className="networks-flow-fields">
                                    <div className="field">
                                      <label>New password</label>
                                      <input
                                        type="password"
                                        autoFocus
                                        value={pskValue}
                                        onChange={(e) => setPskValue(e.target.value)}
                                        placeholder="PSK"
                                      />
                                    </div>
                                  </div>
                                  <div className="networks-drawer-actions">
                                    <button
                                      type="button"
                                      className="button primary sm"
                                      disabled={busy || !pskValue || !canMutate}
                                      onClick={() =>
                                        void mutate("/network/wifi/psk", {
                                          connection: w.connection,
                                          ssid: w.ssid,
                                          psk: pskValue,
                                        }).then(() => {
                                          setPskEdit(null);
                                          setPskValue("");
                                        })
                                      }
                                    >
                                      Save password
                                    </button>
                                    <button type="button" className="button secondary sm" onClick={() => setPskEdit(null)}>
                                      Cancel
                                    </button>
                                  </div>
                                </>
                              ) : (
                                <>
                                  <p className="networks-drawer-note">
                                    {w.active
                                      ? "This is the active association."
                                      : "Connect switches the radio to this profile unless you use Alongside."}
                                  </p>
                                  <div className="networks-drawer-actions">
                                    {canMutate && (
                                      <>
                                        <button
                                          type="button"
                                          className="button primary sm"
                                          disabled={busy}
                                          onClick={() =>
                                            void mutate("/network/wifi/connect", {
                                              connection: w.connection,
                                              ssid: w.ssid,
                                            })
                                          }
                                        >
                                          Connect
                                        </button>
                                        {dualOk && (
                                          <button
                                            type="button"
                                            className="button secondary sm"
                                            disabled={busy}
                                            onClick={() =>
                                              void mutate("/network/wifi/connect", {
                                                connection: w.connection,
                                                ssid: w.ssid,
                                                alongside: true,
                                              })
                                            }
                                          >
                                            Connect alongside
                                          </button>
                                        )}
                                        <button
                                          type="button"
                                          className="button secondary sm"
                                          disabled={busy}
                                          onClick={() => {
                                            setPskEdit(key);
                                            setPskValue("");
                                          }}
                                        >
                                          Change password
                                        </button>
                                        <button
                                          type="button"
                                          className="button secondary sm"
                                          disabled={busy || !w.active}
                                          onClick={() =>
                                            void mutate("/network/wifi/disconnect", {
                                              connection: w.connection,
                                            })
                                          }
                                        >
                                          Disconnect
                                        </button>
                                        <button
                                          type="button"
                                          className="button danger sm"
                                          disabled={busy}
                                          onClick={() => {
                                            if (!confirm(`Forget ${w.ssid}?`)) return;
                                            void mutate("/network/wifi/forget", {
                                              connection: w.connection,
                                              ssid: w.ssid,
                                            });
                                          }}
                                        >
                                          Forget
                                        </button>
                                      </>
                                    )}
                                  </div>
                                </>
                              )}
                            </div>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}

                {wifiFlow && (
                  <div className="networks-flow">
                    <ul className="networks-flow-steps">
                      <li className={wifiStep === "pick" ? "active" : "done"}>1 · Pick</li>
                      <li className={wifiStep === "credentials" ? "active" : wifiStep === "review" ? "done" : ""}>
                        2 · Password
                      </li>
                      <li className={wifiStep === "review" ? "active" : ""}>3 · Confirm</li>
                    </ul>

                    {wifiStep === "pick" && (
                      <>
                        <h3>Choose a network</h3>
                        <p>Nearby SSIDs from a fresh scan. Pick one, or type an SSID below.</p>
                        <div className="networks-flow-footer" style={{ marginTop: 0, marginBottom: 12 }}>
                          <button type="button" className="button secondary sm" disabled={busy} onClick={() => void doWifiScan()}>
                            {busy ? "Scanning…" : "Rescan"}
                          </button>
                        </div>
                        {scan.length > 0 ? (
                          <ul className="networks-scan-list">
                            {scan.map((w) => (
                              <li key={`${w.ssid}-${w.bssid || w.signal}`}>
                                <button
                                  type="button"
                                  className={wifiSSID === w.ssid ? "picked" : ""}
                                  onClick={() => setWifiSSID(w.ssid)}
                                >
                                  <span className="ssid">{w.ssid || "(hidden)"}</span>
                                  <span className="networks-signal">{w.signal ?? "—"}</span>
                                  <span className="sec">{w.security || ""}</span>
                                </button>
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <p className="networks-muted" style={{ marginBottom: 12 }}>
                            {busy ? "Scanning…" : "No results yet — rescan or enter an SSID."}
                          </p>
                        )}
                        <div className="networks-flow-fields">
                          <div className="field">
                            <label>SSID</label>
                            <input value={wifiSSID} onChange={(e) => setWifiSSID(e.target.value)} placeholder="Network name" />
                          </div>
                        </div>
                        <div className="networks-flow-footer">
                          <button type="button" className="button secondary sm" onClick={cancelWifiFlow}>
                            Cancel
                          </button>
                          <button
                            type="button"
                            className="button primary sm"
                            disabled={!wifiSSID}
                            onClick={() => setWifiStep("credentials")}
                          >
                            Continue
                          </button>
                        </div>
                      </>
                    )}

                    {wifiStep === "credentials" && (
                      <>
                        <h3>Password for {wifiSSID}</h3>
                        <p>Saved to the agent without activating — you choose when to connect next.</p>
                        <div className="networks-flow-fields">
                          <div className="field">
                            <label>Wi‑Fi password</label>
                            <input
                              type="password"
                              autoFocus
                              value={wifiPSK}
                              onChange={(e) => setWifiPSK(e.target.value)}
                              placeholder="Leave blank if open"
                            />
                          </div>
                        </div>
                        <div className="networks-flow-footer">
                          <button type="button" className="button secondary sm" onClick={() => setWifiStep("pick")}>
                            Back
                          </button>
                          <button type="button" className="button primary sm" onClick={() => setWifiStep("review")}>
                            Continue
                          </button>
                        </div>
                      </>
                    )}

                    {wifiStep === "review" && (
                      <>
                        <h3>Confirm</h3>
                        <p>
                          Profile <strong>{wifiSSID}</strong> will be saved
                          {wifiConnectAfter
                            ? wifiAlongside && dualOk
                              ? " and connected alongside the current link."
                              : " and brought up (may switch away from the current SSID)."
                            : " only — current link stays up."}
                        </p>
                        <label className="cluster" style={{ gap: 8, marginBottom: 8, fontSize: 13.5 }}>
                          <input
                            type="checkbox"
                            checked={wifiConnectAfter}
                            onChange={(e) => setWifiConnectAfter(e.target.checked)}
                          />
                          Connect after saving
                        </label>
                        {wifiConnectAfter && dualOk && (
                          <label className="cluster" style={{ gap: 8, marginBottom: 8, fontSize: 13.5 }}>
                            <input
                              type="checkbox"
                              checked={wifiAlongside}
                              onChange={(e) => setWifiAlongside(e.target.checked)}
                            />
                            Keep current SSID (alongside)
                          </label>
                        )}
                        <div className="networks-flow-footer">
                          <button type="button" className="button secondary sm" onClick={() => setWifiStep("credentials")}>
                            Back
                          </button>
                          <button
                            type="button"
                            className="button primary sm"
                            disabled={busy || !canMutate}
                            onClick={() => void finishWifiFlow()}
                          >
                            {wifiConnectAfter ? "Save & connect" : "Save network"}
                          </button>
                        </div>
                      </>
                    )}
                  </div>
                )}
              </div>
            </section>
          )}

          {mode === "wired" && (
            <section className="networks-panel">
              <div className="networks-panel-head">
                <div>
                  <h2>Wired</h2>
                  <p>Select an ethernet interface to switch between DHCP and a static address.</p>
                </div>
                {virtualCount > 0 && (
                  <button
                    type="button"
                    className="button secondary sm"
                    onClick={() => setShowVirtual((v) => !v)}
                  >
                    {showVirtual ? "Hide virtual" : `Show virtual (${virtualCount})`}
                  </button>
                )}
              </div>
              <div className="networks-panel-body">
                {ethernet.length === 0 ? (
                  <div className="networks-empty">
                    <strong>No ethernet interfaces</strong>
                    {virtualCount > 0
                      ? "Only virtual links were found — show virtual to configure them."
                      : "This host did not report any ethernet devices."}
                  </div>
                ) : (
                  <ul className="networks-list">
                    {ethernet.map((iface) => {
                      const open = openWired === iface.name;
                      return (
                        <li key={iface.name} className={`networks-item${open ? " open" : ""}`}>
                          <button type="button" className="networks-item-btn" onClick={() => selectWired(iface)}>
                            <div className="networks-item-main">
                              <strong>{iface.name}</strong>
                              <span>
                                {iface.state}
                                {(iface.ipv4 || [])[0] ? ` · ${iface.ipv4![0]}` : ""}
                                {iface.primary ? " · primary route" : ""}
                              </span>
                            </div>
                            <div className="networks-item-side">
                              <Pill ok={iface.state.includes("connected")}>
                                {iface.method === "manual" ? "static" : iface.method || "auto"}
                              </Pill>
                              <Chevron />
                            </div>
                          </button>
                          {open && (
                            <div className="networks-drawer">
                              {iface.primary && (
                                <p className="networks-drawer-note">
                                  This interface carries the default route — changing it can drop the agent.
                                </p>
                              )}
                              <div style={{ marginBottom: 12 }}>
                                <div className="networks-seg" role="group" aria-label="Address method">
                                  <button
                                    type="button"
                                    className={wiredMethod === "dhcp" ? "active" : ""}
                                    onClick={() => setWiredMethod("dhcp")}
                                    disabled={!canMutate}
                                  >
                                    DHCP
                                  </button>
                                  <button
                                    type="button"
                                    className={wiredMethod === "static" ? "active" : ""}
                                    onClick={() => setWiredMethod("static")}
                                    disabled={!canMutate}
                                  >
                                    Static
                                  </button>
                                </div>
                              </div>
                              {wiredMethod === "static" && (
                                <div className="networks-flow-fields two" style={{ maxWidth: "100%", marginBottom: 12 }}>
                                  <div className="field">
                                    <label>Address</label>
                                    <input value={wiredAddress} onChange={(e) => setWiredAddress(e.target.value)} placeholder="192.168.1.50" />
                                  </div>
                                  <div className="field">
                                    <label>Prefix</label>
                                    <input value={wiredPrefix} onChange={(e) => setWiredPrefix(e.target.value)} />
                                  </div>
                                  <div className="field">
                                    <label>Gateway</label>
                                    <input value={wiredGateway} onChange={(e) => setWiredGateway(e.target.value)} />
                                  </div>
                                  <div className="field">
                                    <label>DNS</label>
                                    <input value={wiredDNS} onChange={(e) => setWiredDNS(e.target.value)} placeholder="1.1.1.1, 8.8.8.8" />
                                  </div>
                                </div>
                              )}
                              <div className="networks-drawer-actions">
                                {canMutate && (
                                  <button
                                    type="button"
                                    className="button primary sm"
                                    disabled={busy || (wiredMethod === "static" && !wiredAddress)}
                                    onClick={() =>
                                      void mutate("/network/wired", {
                                        iface: iface.name,
                                        connection: iface.connection,
                                        method: wiredMethod,
                                        address: wiredAddress,
                                        prefix: Number(wiredPrefix) || 24,
                                        gateway: wiredGateway,
                                        dns: wiredDNS.split(",").map((s) => s.trim()).filter(Boolean),
                                      })
                                    }
                                  >
                                    Apply
                                  </button>
                                )}
                                <button type="button" className="button secondary sm" onClick={() => setOpenWired(null)}>
                                  Close
                                </button>
                              </div>
                            </div>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            </section>
          )}

          {mode === "bluetooth" && (
            <section className="networks-panel">
              <div className="networks-panel-head">
                <div>
                  <h2>Bluetooth</h2>
                  <p>
                    Pair and connect devices. PAN tethering uses a NetworkManager bluetooth profile.
                    {!status.bluetooth.available ? " Bluetooth tools are not available on this host." : ""}
                  </p>
                </div>
                {canMutate && status.bluetooth.available && (
                  <button
                    type="button"
                    className="button secondary sm"
                    disabled={busy}
                    onClick={() => void mutate("/network/bluetooth/scan")}
                  >
                    Scan
                  </button>
                )}
              </div>
              <div className="networks-panel-body">
                {!status.bluetooth.available ? (
                  <div className="networks-empty">
                    <strong>Bluetooth unavailable</strong>
                    Install BlueZ / bluetoothctl on the agent host.
                  </div>
                ) : (status.bluetooth.devices || []).length === 0 ? (
                  <div className="networks-empty">
                    <strong>No devices yet</strong>
                    Run a scan to discover nearby hardware.
                  </div>
                ) : (
                  <ul className="networks-list">
                    {(status.bluetooth.devices || []).map((d) => {
                      const open = openBt === d.address;
                      return (
                        <li key={d.address} className={`networks-item${open ? " open" : ""}`}>
                          <button
                            type="button"
                            className="networks-item-btn"
                            onClick={() => setOpenBt(open ? null : d.address)}
                          >
                            <div className="networks-item-main">
                              <strong>{d.name || "Unknown device"}</strong>
                              <span>{d.address}</span>
                            </div>
                            <div className="networks-item-side">
                              {d.connected ? <Pill ok>connected</Pill> : d.paired ? <Pill>paired</Pill> : null}
                              <Chevron />
                            </div>
                          </button>
                          {open && canMutate && (
                            <div className="networks-drawer">
                              <div className="networks-drawer-actions">
                                {!d.paired && (
                                  <button
                                    type="button"
                                    className="button primary sm"
                                    disabled={busy}
                                    onClick={() => void pairBluetooth(d.address)}
                                  >
                                    Pair
                                  </button>
                                )}
                                <button
                                  type="button"
                                  className="button secondary sm"
                                  disabled={busy}
                                  onClick={() => void mutate("/network/bluetooth/connect", { address_bt: d.address })}
                                >
                                  Connect
                                </button>
                                <button
                                  type="button"
                                  className="button secondary sm"
                                  disabled={busy}
                                  onClick={() => void mutate("/network/bluetooth/disconnect", { address_bt: d.address })}
                                >
                                  Disconnect
                                </button>
                                <button
                                  type="button"
                                  className="button secondary sm"
                                  disabled={busy}
                                  onClick={() =>
                                    void mutate("/network/bluetooth/pan", {
                                      action: "connect",
                                      address_bt: d.address,
                                      pan_type: "panu",
                                    })
                                  }
                                >
                                  Start PAN
                                </button>
                                <button
                                  type="button"
                                  className="button danger sm"
                                  disabled={busy}
                                  onClick={() => {
                                    if (!confirm(`Forget ${d.name || d.address}?`)) return;
                                    void mutate("/network/bluetooth/forget", { address_bt: d.address });
                                  }}
                                >
                                  Forget
                                </button>
                              </div>
                            </div>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
                {(status.bluetooth.pan || []).some((p) => p.active) && (
                  <div className="networks-flow">
                    <h3>Active PAN</h3>
                    {(status.bluetooth.pan || [])
                      .filter((p) => p.active)
                      .map((p) => (
                        <div key={p.connection || p.address} className="cluster" style={{ justifyContent: "space-between", marginTop: 8 }}>
                          <span style={{ fontSize: 13.5 }}>
                            {p.name || p.connection} · {(p.ipv4 || []).join(", ") || "no IPv4"}
                          </span>
                          {canMutate && (
                            <button
                              type="button"
                              className="button secondary sm"
                              disabled={busy}
                              onClick={() =>
                                void mutate("/network/bluetooth/pan", {
                                  action: "disconnect",
                                  connection: p.connection,
                                  address_bt: p.address,
                                })
                              }
                            >
                              Disconnect PAN
                            </button>
                          )}
                        </div>
                      ))}
                  </div>
                )}
              </div>
            </section>
          )}

          {mode === "cellular" && (
            <section className="networks-panel">
              <div className="networks-panel-head">
                <div>
                  <h2>Cellular</h2>
                  <p>ModemManager modems and a NetworkManager GSM profile.</p>
                </div>
              </div>
              <div className="networks-panel-body">
                {(status.cellular.modems || []).length === 0 ? (
                  <div className="networks-empty">
                    <strong>No modems detected</strong>
                    Attach a modem or install ModemManager on the agent.
                  </div>
                ) : (
                  <ul className="networks-list">
                    {status.cellular.modems.map((m) => (
                      <li key={m.id} className="networks-item">
                        <div className="networks-item-btn" style={{ cursor: "default" }}>
                          <div className="networks-item-main">
                            <strong>{m.operator || `Modem ${m.id}`}</strong>
                            <span>
                              {m.state || "unknown"}
                              {m.signal != null ? ` · ${m.signal}%` : ""}
                              {m.apn ? ` · ${m.apn}` : ""}
                            </span>
                          </div>
                          <div className="networks-item-side">
                            {m.connected ? <Pill ok>connected</Pill> : <Pill>idle</Pill>}
                          </div>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
                {canMutate && (
                  <div className="networks-flow">
                    <h3>APN & connection</h3>
                    <p>Set the carrier APN, then bring the GSM profile up or down.</p>
                    <div className="networks-flow-fields two">
                      <div className="field">
                        <label>Connection name</label>
                        <input value={cellConn} onChange={(e) => setCellConn(e.target.value)} />
                      </div>
                      <div className="field">
                        <label>APN</label>
                        <input value={cellAPN} onChange={(e) => setCellAPN(e.target.value)} placeholder="internet" />
                      </div>
                    </div>
                    <div className="networks-flow-footer">
                      <button
                        type="button"
                        className="button secondary sm"
                        disabled={busy || !cellAPN}
                        onClick={() => void mutate("/network/cellular/apn", { connection: cellConn, apn: cellAPN })}
                      >
                        Set APN
                      </button>
                      <button
                        type="button"
                        className="button primary sm"
                        disabled={busy}
                        onClick={() =>
                          void mutate("/network/cellular/connect", {
                            connection: cellConn,
                            apn: cellAPN,
                            modem_id: status.cellular.modems[0]?.id,
                          })
                        }
                      >
                        Connect
                      </button>
                      <button
                        type="button"
                        className="button secondary sm"
                        disabled={busy}
                        onClick={() => void mutate("/network/cellular/disconnect", { connection: cellConn })}
                      >
                        Disconnect
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </section>
          )}
        </>
      )}

      {pin && (
        <div className="networks-pin-overlay">
          <div className="networks-pin-modal">
            <h3>Bluetooth {pin.method === "confirm" ? "confirm" : "PIN"}</h3>
            <p>{pin.prompt || `Enter ${pin.method} for ${pin.device}`}</p>
            {pin.method !== "confirm" && (
              <div className="field">
                <label>PIN / passkey</label>
                <input
                  autoFocus
                  value={pinValue}
                  onChange={(e) => setPinValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void submitPin();
                  }}
                />
              </div>
            )}
            <div className="networks-pin-actions">
              <button type="button" className="button secondary" onClick={() => setPin(null)}>
                Cancel
              </button>
              <button type="button" className="button primary" disabled={busy} onClick={() => void submitPin()}>
                {pin.method === "confirm" ? "Confirm" : "Submit"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
