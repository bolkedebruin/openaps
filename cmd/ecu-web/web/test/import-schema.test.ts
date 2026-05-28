import { test, expect, describe } from "bun:test";
import { validateOverlay } from "../src/schemas/overlay.ts";

// A 12-hex uid that satisfies the uid regex.
const U1 = "806000042582";
const U2 = "806000042583";

function ok(r: ReturnType<typeof validateOverlay>) {
  if (!r.ok) throw new Error("expected ok, got errors: " + r.errors.join("; "));
  return r;
}
function err(r: ReturnType<typeof validateOverlay>) {
  if (r.ok) throw new Error("expected errors, got ok");
  return r;
}

describe("validateOverlay — happy paths", () => {
  test("flat point shape normalises", () => {
    const r = ok(validateOverlay({
      id: "site-a",
      uids: [U1],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.profile.id).toBe("site-a");
    expect(r.profile.uids).toEqual([U1]);
    expect(r.profile.points).toHaveLength(1);
    expect(r.profile.points[0]).toEqual({ aps_code: "BP", value: 49.3 });
    expect(r.warnings).toEqual([]);
  });

  test("rich apply/native point shape normalises to flat", () => {
    const r = ok(validateOverlay({
      id: "site-b",
      uids: [U1, U2],
      points: [
        { apply: { aps_code: "BP" }, native: { value: 49.3, unit: "Hz" } },
        { model: 711, group: "x", apply: { aps_code: "CV" }, native: { value: 196, unit: "V" } },
      ],
    }));
    expect(r.profile.points).toEqual([
      { aps_code: "BP", value: 49.3, unit: "Hz" },
      { aps_code: "CV", value: 196, unit: "V" },
    ]);
  });

  test("mixed flat + rich points normalise together", () => {
    const r = ok(validateOverlay({
      id: "mix",
      uids: [U1],
      points: [
        { aps_code: "BP", value: 49.3 },
        { apply: { aps_code: "CV" }, native: { value: 196, unit: "V" } },
      ],
    }));
    expect(r.profile.points.map((p) => p.aps_code)).toEqual(["BP", "CV"]);
  });

  test("schema tag mismatch is a warning, not a rejection", () => {
    const r = ok(validateOverlay({
      schema: "something/else",
      id: "site-w",
      uids: [U1],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.warnings.length).toBeGreaterThan(0);
    expect(r.warnings[0]).toContain("something/else");
    expect(r.warnings[0]).toContain("invdriver.gridprofile/v1");
  });

  test("matching schema tag yields no warnings", () => {
    const r = ok(validateOverlay({
      schema: "invdriver.gridprofile/v1",
      id: "site-w",
      uids: [U1],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.warnings).toEqual([]);
  });

  test("unknown extra fields are dropped silently", () => {
    const r = ok(validateOverlay({
      id: "extras",
      uids: [U1],
      points: [{ aps_code: "BP", value: 49.3, model: 711, range: [40, 70] }],
      vendor_blob: { foo: 1 },
    }));
    expect(r.profile.points[0]).toEqual({ aps_code: "BP", value: 49.3 });
  });
});

describe("validateOverlay — error paths", () => {
  test("missing id", () => {
    const r = err(validateOverlay({
      uids: [U1],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.errors.join("\n")).toMatch(/id/);
  });

  test("empty uids", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.errors.join("\n")).toMatch(/uids/);
  });

  test("empty points", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1],
      points: [],
    }));
    expect(r.errors.join("\n")).toMatch(/points/);
  });

  test("uid wrong length", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: ["abcdef"],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    const text = r.errors.join("\n");
    expect(text).toMatch(/uids\[0\]/);
    expect(text).toMatch(/12 hex/);
  });

  test("uid wrong charset", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: ["80600004258Z"],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.errors.join("\n")).toMatch(/uids\[0\]/);
  });

  test("aps_code lowercase rejected", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1],
      points: [{ aps_code: "bp", value: 49.3 }],
    }));
    const text = r.errors.join("\n");
    expect(text).toMatch(/points\[0\]\.aps_code/);
    expect(text).toMatch(/\^\[A-Z\]\{2\}\$/);
  });

  test("aps_code wrong length rejected", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1],
      points: [{ aps_code: "BPP", value: 49.3 }],
    }));
    expect(r.errors.join("\n")).toMatch(/points\[0\]\.aps_code/);
  });

  test("value NaN rejected", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1],
      points: [{ aps_code: "BP", value: NaN }],
    }));
    expect(r.errors.join("\n")).toMatch(/points\[0\]\.value/);
  });

  test("value Infinity rejected", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1],
      points: [{ aps_code: "BP", value: Infinity }],
    }));
    expect(r.errors.join("\n")).toMatch(/points\[0\]\.value/);
  });

  test("duplicate aps_code rejected", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1],
      points: [
        { aps_code: "BP", value: 49.3 },
        { aps_code: "BP", value: 50.0 },
      ],
    }));
    const text = r.errors.join("\n");
    expect(text).toMatch(/duplicate/);
    expect(text).toMatch(/BP/);
  });

  test("duplicate uid rejected", () => {
    const r = err(validateOverlay({
      id: "x",
      uids: [U1, U1],
      points: [{ aps_code: "BP", value: 49.3 }],
    }));
    expect(r.errors.join("\n")).toMatch(/duplicate/);
  });

  test("multiple errors are all reported (function returns all)", () => {
    const r = err(validateOverlay({
      // id missing, uids empty, point has bad aps_code AND bad value
      uids: [],
      points: [{ aps_code: "bp", value: NaN }],
    }));
    expect(r.errors.length).toBeGreaterThanOrEqual(4);
  });
});
