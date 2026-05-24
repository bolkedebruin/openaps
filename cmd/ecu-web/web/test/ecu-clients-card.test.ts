import { test, expect, describe } from "bun:test";
import "../src/components/ecu-clients-card.ts";
import type { EcuClientsCard } from "../src/components/ecu-clients-card.ts";
import type { SystemStatus } from "../src/api.ts";

async function mount(system: SystemStatus | null): Promise<EcuClientsCard> {
  const el = document.createElement("ecu-clients-card") as EcuClientsCard;
  el.system = system;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

function text(el: HTMLElement): string {
  return el.shadowRoot?.textContent?.replace(/\s+/g, " ").trim() ?? "";
}

const sample: SystemStatus = {
  invdriver_connected: true,
  sse_clients: 1,
  ecu: { ecu_id: "my-roof-ecu", hostname: "ecu" },
  peers: [
    { backend: "ecu-zb", version: "1.0", hostname: "ecu", role: "PUBLISHER", connected_at_ms: 1, peer_uid: 0, controller: false },
    { backend: "ecu-web", version: "0.1", hostname: "ecu", role: "SUBSCRIBER", connected_at_ms: 2, peer_uid: 0, controller: true },
  ],
};

describe("<ecu-clients-card>", () => {
  test("renders ECU identity", async () => {
    const el = await mount(sample);
    const t = text(el);
    expect(t).toContain("my-roof-ecu");
    expect(t).toContain("ecu");
  });

  test("lists peers with roles", async () => {
    const el = await mount(sample);
    expect(el.shadowRoot?.querySelectorAll(".peer").length).toBe(2);
    const t = text(el);
    expect(t).toContain("ecu-zb");
    expect(t).toContain("ecu-web");
  });

  test("controller peer gets a ctrl badge", async () => {
    const el = await mount(sample);
    const ctl = el.shadowRoot?.querySelectorAll(".role.ctl") ?? [];
    expect(ctl.length).toBe(1);
  });

  test("shows status_error when present", async () => {
    const el = await mount({ invdriver_connected: false, sse_clients: 0, peers: [], status_error: "inv-driver down" });
    expect(text(el)).toContain("inv-driver down");
    expect(el.shadowRoot?.querySelector(".warn")).not.toBeNull();
  });

  test("empty peers shows placeholder", async () => {
    const el = await mount({ invdriver_connected: true, sse_clients: 0, peers: [] });
    expect(text(el)).toContain("No peers connected");
  });
});
