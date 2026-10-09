"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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

export default function NetworksPage() {
  const [servers, setServers] = useState<Server[]>([]);
  const [serverId, setServerId] = useState("");
  const [me, setMe] = useState<Me | null>(null);
  const [status, setStatus] = useState<NetworkStatus | null>(null);
  const [scan, setScan] = useState<WifiNet[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [wiredIface, setWiredIface] = useState("");
  const [wiredMethod, setWiredMethod] = useState("dhcp");
  const [wiredAddress, setWiredAddress] = useState("");
  const [wiredPrefix, setWiredPrefix] = useState("24");
  const [wiredGateway, setWiredGateway] = useState("");
  const [wiredDNS, setWiredDNS] = useState("");

  const [wifiSSID, setWifiSSID] = useState("");
  const [wifiPSK, setWifiPSK] = useState("");
  const [pskTarget, setPskTarget] = useState("");
  const [pskValue, setPskValue] = useState("");

  const [cellAPN, setCellAPN] = useState("");
  const [cellConn, setCellConn] = useState("cc-cellular");

  const [pin, setPin] = useState<PinChallenge | null>(null);
  const [pinValue, setPinValue] = useState("");

  const canMutate = me?.role === "admin" || me?.role === "owner";

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
      setError(typeof body.error === "string" ? body.error : `HTTP error`);
      return;
    }
    const st = (body.result || body) as NetworkStatus;
    setStatus(st);
    const eth = (st.interfaces || []).find((i) => i.type === "ethernet");
    if (eth && !wiredIface) {
      setWiredIface(eth.name);
      setWiredMethod(eth.method === "manual" ? "static" : "dhcp");
      if (eth.ipv4?.[0]) {
        const [addr, pref] = eth.ipv4[0].split("/");
        setWiredAddress(addr || "");
        setWiredPrefix(pref || "24");
      }
      setWiredGateway(eth.gateway || "");
      setWiredDNS((eth.dns || []).join(", "));
    }
  }, [serverId, wiredIface]);

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
      let failed = false;
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
          if (event === "done") {
            if (payload.status === "failed") {
              failed = true;
              throw new Error(payload.error || "pair failed");
            }
          }
        }
      }
      if (!failed) await refresh();
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

  const ethernet = (status?.interfaces || []).filter((i) => i.type === "ethernet");
  const dualOk = !!status?.dual_wifi?.supported;

  return (
    <div className="page networks-page">
      <div className="page-head">
        <div>
          <h1><IconNetwork /> Networks</h1>
          <p className="muted">
            Wired, Wi‑Fi, Bluetooth, and cellular on a selected agent. Uses NetworkManager on
            Raspberry Pi OS when available.
          </p>
        </div>
        {status && (
          <span className="networks-badge">
            backend · {status.backend}
            {status.control_plane_iface ? ` · via ${status.control_plane_iface}` : ""}
          </span>
        )}
      </div>

      <div className="panel networks-selectors">
        <div className="field">
          <label>Server</label>
          <SearchableSelect
            options={serverOptions}
            value={serverId}
            onChange={setServerId}
            placeholder="Select a server"
          />
        </div>
        <div className="networks-form-actions" style={{ marginTop: 12 }}>
          <button type="button" className="button" onClick={() => void refresh()} disabled={!serverId || busy}>
            Refresh
          </button>
          {!canMutate && (
            <span className="networks-meta">View only — admin required to change networks</span>
          )}
        </div>
      </div>

      {error && <div className="flash error" style={{ marginTop: 12 }}>{error}</div>}
      {!serverId && <p className="networks-empty">Select a server to manage its networks.</p>}
      {serverId && !status && !error && <p className="networks-empty">Loading…</p>}

      {status && (
        <div className="networks-sections">
          {/* Wired */}
          <section className="panel networks-section">
            <h3>Wired</h3>
            <p className="section-blurb">DHCP or static IPv4 on ethernet interfaces.</p>
            {ethernet.length === 0 ? (
              <p className="networks-empty">No ethernet interfaces reported.</p>
            ) : (
              <table className="networks-table">
                <thead>
                  <tr>
                    <th>Interface</th>
                    <th>State</th>
                    <th>IPv4</th>
                    <th>Gateway</th>
                    <th>Method</th>
                  </tr>
                </thead>
                <tbody>
                  {ethernet.map((i) => (
                    <tr key={i.name}>
                      <td>
                        {i.name}
                        {i.primary ? " · primary" : ""}
                      </td>
                      <td>{i.state}</td>
                      <td>{(i.ipv4 || []).join(", ") || "—"}</td>
                      <td>{i.gateway || "—"}</td>
                      <td>{i.method || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {canMutate && (
              <div className="networks-form">
                <div className="field">
                  <label>Interface</label>
                  <select value={wiredIface} onChange={(e) => setWiredIface(e.target.value)}>
                    {ethernet.map((i) => (
                      <option key={i.name} value={i.name}>{i.name}</option>
                    ))}
                  </select>
                </div>
                <div className="field">
                  <label>Method</label>
                  <select value={wiredMethod} onChange={(e) => setWiredMethod(e.target.value)}>
                    <option value="dhcp">DHCP</option>
                    <option value="static">Static</option>
                  </select>
                </div>
                {wiredMethod === "static" && (
                  <>
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
                  </>
                )}
                <div className="networks-form-actions">
                  <button
                    type="button"
                    className="button primary"
                    disabled={busy || !wiredIface}
                    onClick={() =>
                      void mutate("/network/wired", {
                        iface: wiredIface,
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
                </div>
              </div>
            )}
          </section>

          {/* Wireless */}
          <section className="panel networks-section">
            <h3>Wireless</h3>
            <p className="section-blurb">
              Save profiles without dropping the current link. Update passwords in place.
              {dualOk
                ? " Dual Wi‑Fi is available — Connect alongside keeps the current SSID."
                : status.dual_wifi?.detail
                  ? ` Dual Wi‑Fi: ${status.dual_wifi.detail}.`
                  : ""}
            </p>
            {status.wifi.active && (
              <p className="networks-meta">
                Active: <span className="networks-active">{status.wifi.active.ssid}</span>
                {status.wifi.active.signal != null ? ` · ${status.wifi.active.signal}%` : ""}
              </p>
            )}
            <table className="networks-table">
              <thead>
                <tr>
                  <th>Saved SSID</th>
                  <th>Connection</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {(status.wifi.saved || []).length === 0 ? (
                  <tr><td colSpan={3} className="networks-empty">No saved Wi‑Fi profiles.</td></tr>
                ) : (
                  (status.wifi.saved || []).map((w) => (
                    <tr key={w.connection || w.ssid}>
                      <td>
                        {w.ssid}
                        {w.active ? <span className="networks-active"> · active</span> : null}
                      </td>
                      <td>{w.connection || "—"}</td>
                      <td>
                        {canMutate && (
                          <div className="networks-row-actions">
                            <button
                              type="button"
                              className="button sm"
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
                                className="button sm"
                                disabled={busy}
                                onClick={() =>
                                  void mutate("/network/wifi/connect", {
                                    connection: w.connection,
                                    ssid: w.ssid,
                                    alongside: true,
                                  })
                                }
                              >
                                Alongside
                              </button>
                            )}
                            <button
                              type="button"
                              className="button sm"
                              disabled={busy}
                              onClick={() => {
                                setPskTarget(w.connection || w.ssid);
                                setPskValue("");
                              }}
                            >
                              Password
                            </button>
                            <button
                              type="button"
                              className="button sm"
                              disabled={busy}
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
                              className="button sm danger"
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
                          </div>
                        )}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>

            {canMutate && pskTarget && (
              <div className="networks-form">
                <div className="field">
                  <label>Update password for {pskTarget}</label>
                  <input
                    type="password"
                    value={pskValue}
                    onChange={(e) => setPskValue(e.target.value)}
                    placeholder="New PSK"
                  />
                </div>
                <div className="networks-form-actions">
                  <button
                    type="button"
                    className="button primary"
                    disabled={busy || !pskValue}
                    onClick={() =>
                      void mutate("/network/wifi/psk", {
                        connection: pskTarget,
                        psk: pskValue,
                      }).then(() => {
                        setPskTarget("");
                        setPskValue("");
                      })
                    }
                  >
                    Save password
                  </button>
                  <button type="button" className="button" onClick={() => setPskTarget("")}>
                    Cancel
                  </button>
                </div>
              </div>
            )}

            {canMutate && (
              <div className="networks-form">
                <div className="field">
                  <label>SSID</label>
                  <input value={wifiSSID} onChange={(e) => setWifiSSID(e.target.value)} />
                </div>
                <div className="field">
                  <label>Password</label>
                  <input type="password" value={wifiPSK} onChange={(e) => setWifiPSK(e.target.value)} />
                </div>
                <div className="networks-form-actions">
                  <button
                    type="button"
                    className="button primary"
                    disabled={busy || !wifiSSID}
                    onClick={() =>
                      void mutate("/network/wifi/save", { ssid: wifiSSID, psk: wifiPSK }).then(() => {
                        setWifiSSID("");
                        setWifiPSK("");
                      })
                    }
                  >
                    Save network
                  </button>
                  <button type="button" className="button" disabled={busy} onClick={() => void doWifiScan()}>
                    Scan
                  </button>
                </div>
              </div>
            )}

            {scan.length > 0 && (
              <table className="networks-table" style={{ marginTop: 12 }}>
                <thead>
                  <tr>
                    <th>Nearby SSID</th>
                    <th>Signal</th>
                    <th>Security</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {scan.map((w) => (
                    <tr key={`${w.ssid}-${w.bssid || w.signal}`}>
                      <td>{w.ssid || "(hidden)"}</td>
                      <td>{w.signal ?? "—"}</td>
                      <td>{w.security || "—"}</td>
                      <td>
                        {canMutate && w.ssid && (
                          <button
                            type="button"
                            className="button sm"
                            disabled={busy}
                            onClick={() => setWifiSSID(w.ssid)}
                          >
                            Use
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>

          {/* Bluetooth */}
          <section className="panel networks-section">
            <h3>Bluetooth</h3>
            <p className="section-blurb">
              Pair, connect, disconnect, and forget devices. PAN tethering uses NetworkManager
              bluetooth profiles.
              {!status.bluetooth.available ? " Bluetooth tools not available on this host." : ""}
            </p>
            {status.bluetooth.available && (
              <>
                <div className="networks-form-actions" style={{ marginBottom: 10 }}>
                  {canMutate && (
                    <button type="button" className="button" disabled={busy} onClick={() => void mutate("/network/bluetooth/scan")}>
                      Scan devices
                    </button>
                  )}
                  <span className="networks-meta">
                    Adapter {status.bluetooth.powered ? "powered on" : "powered off"}
                  </span>
                </div>
                <table className="networks-table">
                  <thead>
                    <tr>
                      <th>Device</th>
                      <th>Address</th>
                      <th>State</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {(status.bluetooth.devices || []).length === 0 ? (
                      <tr><td colSpan={4} className="networks-empty">No devices. Run a scan.</td></tr>
                    ) : (
                      (status.bluetooth.devices || []).map((d) => (
                        <tr key={d.address}>
                          <td>{d.name || "Unknown"}</td>
                          <td>{d.address}</td>
                          <td>
                            {[d.connected && "connected", d.paired && "paired", d.trusted && "trusted"]
                              .filter(Boolean)
                              .join(", ") || "—"}
                          </td>
                          <td>
                            {canMutate && (
                              <div className="networks-row-actions">
                                {!d.paired && (
                                  <button type="button" className="button sm" disabled={busy} onClick={() => void pairBluetooth(d.address)}>
                                    Pair
                                  </button>
                                )}
                                <button
                                  type="button"
                                  className="button sm"
                                  disabled={busy}
                                  onClick={() => void mutate("/network/bluetooth/connect", { address_bt: d.address })}
                                >
                                  Connect
                                </button>
                                <button
                                  type="button"
                                  className="button sm"
                                  disabled={busy}
                                  onClick={() => void mutate("/network/bluetooth/disconnect", { address_bt: d.address })}
                                >
                                  Disconnect
                                </button>
                                <button
                                  type="button"
                                  className="button sm"
                                  disabled={busy}
                                  onClick={() =>
                                    void mutate("/network/bluetooth/pan", {
                                      action: "connect",
                                      address_bt: d.address,
                                      pan_type: "panu",
                                    })
                                  }
                                >
                                  PAN
                                </button>
                                <button
                                  type="button"
                                  className="button sm danger"
                                  disabled={busy}
                                  onClick={() => {
                                    if (!confirm(`Forget ${d.name || d.address}?`)) return;
                                    void mutate("/network/bluetooth/forget", { address_bt: d.address });
                                  }}
                                >
                                  Forget
                                </button>
                              </div>
                            )}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
                {(status.bluetooth.pan || []).length > 0 && (
                  <>
                    <h3 style={{ marginTop: 16 }}>PAN links</h3>
                    <table className="networks-table">
                      <thead>
                        <tr>
                          <th>Name</th>
                          <th>Address</th>
                          <th>IPv4</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {status.bluetooth.pan.map((p) => (
                          <tr key={p.connection || p.address}>
                            <td>{p.name || p.connection}</td>
                            <td>{p.address}</td>
                            <td>{(p.ipv4 || []).join(", ") || "—"}</td>
                            <td>
                              {canMutate && p.active && (
                                <button
                                  type="button"
                                  className="button sm"
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
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </>
                )}
              </>
            )}
          </section>

          {/* Cellular */}
          {(status.cellular.available || (status.cellular.modems || []).length > 0) && (
            <section className="panel networks-section">
              <h3>Cellular</h3>
              <p className="section-blurb">ModemManager modems and NetworkManager GSM connections.</p>
              {(status.cellular.modems || []).length === 0 ? (
                <p className="networks-empty">No modems detected.</p>
              ) : (
                <table className="networks-table">
                  <thead>
                    <tr>
                      <th>Modem</th>
                      <th>Operator</th>
                      <th>Signal</th>
                      <th>State</th>
                      <th>APN</th>
                    </tr>
                  </thead>
                  <tbody>
                    {status.cellular.modems.map((m) => (
                      <tr key={m.id}>
                        <td>{m.id}</td>
                        <td>{m.operator || "—"}</td>
                        <td>{m.signal != null ? `${m.signal}%` : "—"}</td>
                        <td>{m.connected ? "connected" : m.state || "—"}</td>
                        <td>{m.apn || "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              {canMutate && (
                <div className="networks-form">
                  <div className="field">
                    <label>Connection name</label>
                    <input value={cellConn} onChange={(e) => setCellConn(e.target.value)} />
                  </div>
                  <div className="field">
                    <label>APN</label>
                    <input value={cellAPN} onChange={(e) => setCellAPN(e.target.value)} placeholder="internet" />
                  </div>
                  <div className="networks-form-actions">
                    <button
                      type="button"
                      className="button"
                      disabled={busy || !cellAPN}
                      onClick={() => void mutate("/network/cellular/apn", { connection: cellConn, apn: cellAPN })}
                    >
                      Set APN
                    </button>
                    <button
                      type="button"
                      className="button primary"
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
                      className="button"
                      disabled={busy}
                      onClick={() => void mutate("/network/cellular/disconnect", { connection: cellConn })}
                    >
                      Disconnect
                    </button>
                  </div>
                </div>
              )}
            </section>
          )}
        </div>
      )}

      {servers.length === 0 && (
        <p className="networks-empty" style={{ marginTop: 16 }}>
          No servers yet. <Link href="/servers/new">Enroll an agent</Link> first.
        </p>
      )}

      {pin && (
        <div className="networks-pin-overlay">
          <div className="networks-pin-modal">
            <h3>Bluetooth {pin.method === "confirm" ? "confirm" : "PIN"}</h3>
            <p>
              {pin.prompt || `Enter ${pin.method} for ${pin.device}`}
            </p>
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
              <button type="button" className="button" onClick={() => setPin(null)}>
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
