import { describe, expect, it } from "vitest";
import { attendanceSummary, normalizeSlotPosition } from "./domain";

describe("attendanceSummary", () => {
  it("summarizes available and unavailable player counts", () => {
    const summary = attendanceSummary([
      { player_id: "p1", status: "available", note: "", updated_at: "" },
      { player_id: "p2", status: "late", note: "", updated_at: "" },
      { player_id: "p3", status: "injured", note: "", updated_at: "" },
    ]);

    expect(summary.ready).toBe(2);
    expect(summary.blocked).toBe(1);
  });
});

describe("normalizeSlotPosition", () => {
  it("keeps dragged tactic slots inside the pitch", () => {
    expect(normalizeSlotPosition({ x: -4, y: 112 })).toEqual({ x: 0, y: 100 });
  });
});
