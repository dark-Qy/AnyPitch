import { describe, expect, it } from "vitest";
import {
  attendanceSummary,
  buildMonthCalendar,
  groupEventsByDate,
  opponentSlotsFromTemplate,
  pitchGeometry,
  templatesForFormat,
  normalizeSlotPosition,
} from "./domain";

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
    expect(normalizeSlotPosition({ x: -4, y: 112 })).toEqual({ x: 4, y: 96 });
  });
});

describe("buildMonthCalendar", () => {
  it("builds a Monday-first six-week month grid", () => {
    const days = buildMonthCalendar(2026, 4, new Date(2026, 4, 11));

    expect(days).toHaveLength(42);
    expect(days[0].key).toBe("2026-04-27");
    expect(days[4].key).toBe("2026-05-01");
    expect(days.find((day) => day.key === "2026-05-11")?.isToday).toBe(true);
  });
});

describe("groupEventsByDate", () => {
  it("groups events by local calendar day", () => {
    const grouped = groupEventsByDate([
      { starts_at: "2026-05-13T20:00:00+08:00", title: "训练" },
      { starts_at: "2026-05-13T21:00:00+08:00", title: "加练" },
      { starts_at: "2026-05-16T18:00:00+08:00", title: "友谊赛" },
    ]);

    expect(grouped["2026-05-13"]).toHaveLength(2);
    expect(grouped["2026-05-16"][0].title).toBe("友谊赛");
  });
});

describe("tactic helpers", () => {
  const templates = [
    { id: "f5", format: 5 as const, slots: [{ slot_id: "gk", label: "门将", x: 50, y: 91 }] },
    { id: "f8", format: 8 as const, slots: [{ slot_id: "gk", label: "门将", x: 50, y: 93 }] },
  ];

  it("filters templates by format", () => {
    expect(templatesForFormat(templates, 5).map((template) => template.id)).toEqual(["f5"]);
  });

  it("mirrors opponent slots to the opposite half", () => {
    expect(opponentSlotsFromTemplate(templates[0])[0]).toMatchObject({
      slot_id: "opponent:gk",
      side: "opponent",
      x: 50,
      y: 9,
    });
  });

  it("nudges mirrored opponent midfielders away from direct overlap", () => {
    const mirrored = opponentSlotsFromTemplate({
      id: "f8",
      format: 8,
      slots: [{ slot_id: "cm", label: "中场", x: 50, y: 50 }],
    });

    expect(mirrored[0]).toMatchObject({ slot_id: "opponent:cm", side: "opponent", x: 50, y: 42 });
  });

  it("selects a realistic vertical pitch ratio", () => {
    expect(pitchGeometry(5)).toEqual({ ratio: "20 / 40", maxWidth: "460px" });
    expect(pitchGeometry(11)).toEqual({ ratio: "68 / 105", maxWidth: "620px" });
  });
});
