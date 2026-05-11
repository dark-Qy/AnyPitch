import { describe, expect, it } from "vitest";
import { attendanceSummary, buildMonthCalendar, groupEventsByDate, normalizeSlotPosition } from "./domain";

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
