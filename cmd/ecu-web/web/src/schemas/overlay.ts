// Schema validation for the overlay-import JSON.
//
// Accepts both the flat shape ({aps_code, value, unit?}) the API uses and
// the rich shape ({apply:{aps_code}, native:{value, unit?}}) a hand-authored
// or base-profile-export file uses. The schema normalises both into the
// flat LocalSiteProfile shape the editor expects.

import * as z from "zod/mini";
import type { LocalSiteProfile, OverlayPoint } from "../api.ts";

const SCHEMA_TAG = "invdriver.gridprofile/v1";

const ApsCode = z.string().check(
  z.regex(/^[A-Z]{2}$/, "must match ^[A-Z]{2}$"),
);

const Uid = z.string().check(
  z.regex(/^[0-9A-Fa-f]{12}$/, "must be 12 hex characters"),
);

// A point is either flat or rich. A transform flattens both into the same
// downstream shape so a single object schema validates the result, and
// per-field errors report under .aps_code / .value rather than as a noisy
// "neither union branch matched" error. (Mini's equivalent of classic
// preprocess is pipe(transform, schema).)
const PointSchema = z.pipe(
  z.transform((raw: unknown) => {
    if (raw && typeof raw === "object") {
      const r = raw as Record<string, unknown>;
      const hasFlat = "aps_code" in r || "value" in r;
      const hasRich = "apply" in r || "native" in r;
      if (hasFlat) {
        return { aps_code: r.aps_code, value: r.value, unit: r.unit };
      }
      if (hasRich) {
        const apply = (r.apply ?? {}) as Record<string, unknown>;
        const native = (r.native ?? {}) as Record<string, unknown>;
        return { aps_code: apply.aps_code, value: native.value, unit: native.unit };
      }
    }
    return raw;
  }),
  z.object({
    aps_code: ApsCode,
    value: z.number(), // mini's z.number() already rejects NaN/Infinity
    unit: z.optional(z.string()),
  }),
);

const OverlayObject = z.object({
  // schema is optional; a non-matching value is a warning, not a rejection.
  schema: z.optional(z.string()),
  id: z.string().check(z.trim(), z.minLength(1, "must be a non-empty string")),
  uids: z.array(Uid).check(z.minLength(1, "must contain at least one inverter UID")),
  points: z.array(PointSchema).check(z.minLength(1, "must contain at least one parameter override")),
});

// Mini doesn't re-export a superRefine helper, so we attach a custom check
// that pushes raw issues directly to the payload — same effect.
export const OverlaySchema = OverlayObject.check(
  z.check<z.infer<typeof OverlayObject>>((payload) => {
    const v = payload.value;
    // Duplicate aps_code across points is an operator mistake.
    const seenCodes = new Map<string, number>();
    for (let i = 0; i < v.points.length; i++) {
      const code = v.points[i].aps_code;
      const prev = seenCodes.get(code);
      if (prev !== undefined) {
        payload.issues.push({
          code: "custom",
          path: ["points", i, "aps_code"],
          message: `duplicate aps_code "${code}" (also at points[${prev}])`,
          input: v,
        });
      } else {
        seenCodes.set(code, i);
      }
    }
    // Duplicate uid is likewise an operator mistake.
    const seenUids = new Map<string, number>();
    for (let i = 0; i < v.uids.length; i++) {
      const u = v.uids[i].toLowerCase();
      const prev = seenUids.get(u);
      if (prev !== undefined) {
        payload.issues.push({
          code: "custom",
          path: ["uids", i],
          message: `duplicate uid "${v.uids[i]}" (also at uids[${prev}])`,
          input: v,
        });
      } else {
        seenUids.set(u, i);
      }
    }
  }),
);

export type OverlayImport = z.infer<typeof OverlaySchema>;

/** Format a Zod issue path as "points[2].aps_code" / "uids[0]" / "id". */
function formatPath(path: ReadonlyArray<PropertyKey>): string {
  let out = "";
  for (const seg of path) {
    if (typeof seg === "number") {
      out += `[${seg}]`;
    } else {
      out += out ? `.${String(seg)}` : String(seg);
    }
  }
  return out || "(root)";
}

export type ValidateResult =
  | { ok: true; profile: LocalSiteProfile; warnings: string[] }
  | { ok: false; errors: string[] };

/**
 * Validate and normalise a parsed overlay JSON object. On success the
 * returned profile is the flat {aps_code, value, unit?} shape the editor
 * uses; rich-shape points are flattened automatically. Warnings cover
 * non-rejecting issues such as a schema tag that doesn't match this build.
 */
export function validateOverlay(raw: unknown): ValidateResult {
  const r = z.safeParse(OverlaySchema, raw);
  if (!r.success) {
    const errors = r.error.issues.map((iss) => `${formatPath(iss.path)}: ${iss.message}`);
    return { ok: false, errors };
  }
  const warnings: string[] = [];
  if (r.data.schema !== undefined && r.data.schema !== SCHEMA_TAG) {
    warnings.push(`schema tag "${r.data.schema}" does not match expected "${SCHEMA_TAG}"`);
  }
  const points: OverlayPoint[] = r.data.points.map((p) => {
    const out: OverlayPoint = { aps_code: p.aps_code, value: p.value };
    if (p.unit !== undefined && p.unit !== "") out.unit = p.unit;
    return out;
  });
  const profile: LocalSiteProfile = {
    id: r.data.id.trim(),
    uids: r.data.uids,
    points,
  };
  return { ok: true, profile, warnings };
}
