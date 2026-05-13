import { describe, expect, it } from "vitest";
import {
  activeRosterPlayers,
  attendanceDecisionSummary,
  attendanceSummary,
  buildAttendanceStatusMap,
  buildMonthCalendar,
  buildPlayerEventStatusMap,
  calendarEventStatusClass,
  calendarEventStatusLabel,
  eventTimeRange,
  groupEventsByDate,
  initialCoachLoginDraft,
  nextSelectedEventID,
  opponentSlotsFromTemplate,
  pitchGeometry,
  positionAfterDragDelta,
  splitEventsByTime,
  templatesForFormat,
  normalizeSlotPosition,
  toLocalDateTimeInput,
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

describe("attendanceDecisionSummary", () => {
  it("separates attending, declined, tentative, and unseen players", () => {
    expect(
      attendanceDecisionSummary(
        [
          { player_id: "p1", status: "available", note: "", updated_at: "" },
          { player_id: "p2", status: "unavailable", note: "", updated_at: "" },
          { player_id: "p3", status: "tentative", note: "", updated_at: "" },
          { player_id: "p4", status: "unknown", note: "", updated_at: "" },
        ],
        5,
      ),
    ).toEqual({ available: 1, unavailable: 1, tentative: 1, unknown: 2 });
  });
});

describe("activeRosterPlayers", () => {
  it("keeps inactive players out of attendance totals", () => {
    const players = activeRosterPlayers([
      { id: "p1", name: "林海", status: "active" },
      { id: "p2", name: "周舟", status: "inactive" },
    ]);

    expect(players.map((player) => player.id)).toEqual(["p1"]);
  });
});

describe("buildAttendanceStatusMap", () => {
  it("fills every scheduled player and overlays saved records", () => {
    const records = buildAttendanceStatusMap(
      [
        { id: "p1", name: "林海" },
        { id: "p2", name: "周舟" },
      ],
      [{ player_id: "p2", status: "late", note: "", updated_at: "" }],
    );

    expect(records).toEqual({ p1: "unknown", p2: "late" });
  });

  it("ignores saved records for players outside the current attendance roster", () => {
    const records = buildAttendanceStatusMap(
      [{ id: "p1", name: "林海" }],
      [
        { player_id: "p1", status: "available", note: "", updated_at: "" },
        { player_id: "p2", status: "late", note: "", updated_at: "" },
      ],
    );

    expect(records).toEqual({ p1: "available" });
  });
});

describe("nextSelectedEventID", () => {
  const events = [{ id: "e1" }, { id: "e2" }];

  it("keeps the current event when it still exists after a refresh", () => {
    expect(nextSelectedEventID("e2", events)).toBe("e2");
  });

  it("selects the first refreshed event when the current event is missing", () => {
    expect(nextSelectedEventID("stale", events)).toBe("e1");
  });
});

describe("initialCoachLoginDraft", () => {
  it("only keeps an empty coach password draft", () => {
    expect(initialCoachLoginDraft()).toEqual({ password: "" });
  });
});

describe("normalizeSlotPosition", () => {
  it("keeps dragged tactic slots inside the pitch", () => {
    expect(normalizeSlotPosition({ x: -4, y: 112 })).toEqual({ x: 4, y: 96 });
  });
});

describe("positionAfterDragDelta", () => {
  it("converts drag pixels into percentage movement from the current slot", () => {
    expect(positionAfterDragDelta({ x: 50, y: 40 }, { x: 34, y: 105 }, { width: 340, height: 700 })).toEqual({
      x: 60,
      y: 55,
    });
  });

  it("falls back to the current normalized slot when the pitch size is invalid", () => {
    expect(positionAfterDragDelta({ x: -8, y: 120 }, { x: 40, y: 40 }, { width: 0, height: 700 })).toEqual({ x: 4, y: 96 });
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

describe("player event attendance helpers", () => {
  const events = [
    { id: "past", starts_at: "2026-05-01T20:00:00+08:00", ends_at: "2026-05-01T22:00:00+08:00" },
    { id: "future", starts_at: "2026-05-13T20:30:00+08:00", ends_at: "2026-05-13T22:00:00+08:00" },
    { id: "unknown", starts_at: "2026-05-16T18:00:00+08:00", ends_at: "2026-05-16T20:00:00+08:00" },
  ];

  it("maps every event to the current player's status and defaults missing records to unknown", () => {
    const statusByEvent = buildPlayerEventStatusMap(events, [
      { event_id: "past", player_id: "p1", status: "available", note: "", updated_at: "" },
      { event_id: "future", player_id: "p1", status: "tentative", note: "", updated_at: "" },
      { event_id: "outside", player_id: "p1", status: "unavailable", note: "", updated_at: "" },
    ]);

    expect(statusByEvent).toEqual({
      past: "available",
      future: "tentative",
      unknown: "unknown",
    });
  });

  it("separates upcoming events from finished events without mutating source order", () => {
    const split = splitEventsByTime(events, new Date("2026-05-12T12:00:00+08:00"));

    expect(split.upcoming.map((event) => event.id)).toEqual(["future", "unknown"]);
    expect(split.past.map((event) => event.id)).toEqual(["past"]);
    expect(events.map((event) => event.id)).toEqual(["past", "future", "unknown"]);
  });

  it("exposes stable calendar status classes and labels", () => {
    expect(calendarEventStatusClass("available")).toBe("status-available");
    expect(calendarEventStatusClass("unavailable")).toBe("status-unavailable");
    expect(calendarEventStatusClass("tentative")).toBe("status-tentative");
    expect(calendarEventStatusClass("unknown")).toBe("status-unknown");
    expect(calendarEventStatusLabel("tentative")).toBe("待定");
  });

  it("formats event time ranges for compact calendar chips", () => {
    expect(eventTimeRange(events[1])).toBe("20:30-22:00");
  });

  it("formats server event times for datetime-local inputs", () => {
    const localDate = new Date(2026, 4, 13, 20, 30);

    expect(toLocalDateTimeInput(localDate.toISOString())).toBe("2026-05-13T20:30");
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
    expect(pitchGeometry(5)).toEqual({ ratio: "68 / 105", maxWidth: "560px" });
    expect(pitchGeometry(11)).toEqual({ ratio: "68 / 105", maxWidth: "620px" });
  });
});
