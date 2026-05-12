export type AttendanceStatus =
  | "unknown"
  | "available"
  | "unavailable"
  | "tentative"
  | "late"
  | "injured"
  | "present"
  | "absent"
  | "excused";

export type AttendanceRecord = {
  player_id: string;
  status: AttendanceStatus;
  note: string;
  updated_at: string;
};

export type AttendanceDecisionSummary = {
  available: number;
  unavailable: number;
  tentative: number;
  unknown: number;
};

export type SlotPosition = {
  x: number;
  y: number;
};

export type TacticFormat = 5 | 8 | 11;
export type SlotSide = "home" | "opponent";

export type TemplateLike = {
  id: string;
  format: TacticFormat;
  slots: Array<SlotPosition & { slot_id: string; label: string; side?: SlotSide; player_id?: string }>;
};

export type CalendarDay = {
  key: string;
  date: Date;
  dayOfMonth: number;
  inCurrentMonth: boolean;
  isToday: boolean;
};

export function initialCoachLoginDraft() {
  return { email: "", password: "" };
}

export function buildAttendanceStatusMap<T extends { id: string }>(
  players: T[],
  records: AttendanceRecord[],
): Record<string, AttendanceStatus> {
  const next: Record<string, AttendanceStatus> = {};
  for (const player of players) {
    next[player.id] = "unknown";
  }
  for (const record of records) {
    next[record.player_id] = record.status;
  }
  return next;
}

export function attendanceSummary(records: AttendanceRecord[]) {
  return records.reduce(
    (summary, record) => {
      if (record.status === "available" || record.status === "late" || record.status === "present") {
        summary.ready += 1;
      }
      if (record.status === "unavailable" || record.status === "injured" || record.status === "absent") {
        summary.blocked += 1;
      }
      return summary;
    },
    { ready: 0, blocked: 0 },
  );
}

export function attendanceDecisionSummary(records: AttendanceRecord[], totalPlayers: number): AttendanceDecisionSummary {
  const summary: AttendanceDecisionSummary = {
    available: 0,
    unavailable: 0,
    tentative: 0,
    unknown: 0,
  };
  for (const record of records) {
    if (record.status === "available" || record.status === "late" || record.status === "present") {
      summary.available += 1;
    } else if (record.status === "unavailable" || record.status === "injured" || record.status === "absent" || record.status === "excused") {
      summary.unavailable += 1;
    } else if (record.status === "tentative") {
      summary.tentative += 1;
    }
  }
  const known = summary.available + summary.unavailable + summary.tentative;
  summary.unknown = Math.max(0, totalPlayers - known);
  return summary;
}

export function normalizeSlotPosition(position: SlotPosition): SlotPosition {
  return {
    x: clamp(position.x),
    y: clamp(position.y),
  };
}

export function positionAfterDragDelta(
  current: SlotPosition,
  delta: SlotPosition,
  pitchSize: { width: number; height: number },
): SlotPosition {
  if (pitchSize.width <= 0 || pitchSize.height <= 0) {
    return normalizeSlotPosition(current);
  }

  return normalizeSlotPosition({
    x: current.x + (delta.x / pitchSize.width) * 100,
    y: current.y + (delta.y / pitchSize.height) * 100,
  });
}

export function templatesForFormat<T extends { format: TacticFormat }>(templates: T[], format: TacticFormat): T[] {
  return templates.filter((template) => template.format === format);
}

export function opponentSlotsFromTemplate<T extends TemplateLike>(template: T) {
  return template.slots.map((slot) => ({
    ...slot,
    slot_id: `opponent:${stripSlotSide(slot.slot_id)}`,
    side: "opponent" as const,
    x: clamp(100 - slot.x),
    y: opponentMirrorY(slot.y),
    player_id: "",
  }));
}

export function homeSlotsFromTemplate<T extends TemplateLike>(template: T) {
  return template.slots.map((slot) => ({
    ...slot,
    slot_id: `home:${stripSlotSide(slot.slot_id)}`,
    side: "home" as const,
    player_id: "",
  }));
}

export function pitchGeometry(format: TacticFormat) {
  if (format === 5) {
    return { ratio: "68 / 105", maxWidth: "560px" };
  }
  return { ratio: "68 / 105", maxWidth: "620px" };
}

export function buildMonthCalendar(year: number, month: number, today = new Date()): CalendarDay[] {
  const firstOfMonth = new Date(year, month, 1);
  const mondayOffset = (firstOfMonth.getDay() + 6) % 7;
  const start = new Date(year, month, 1 - mondayOffset);
  const todayKey = toLocalDateKey(today);

  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start.getFullYear(), start.getMonth(), start.getDate() + index);
    const key = toLocalDateKey(date);
    return {
      key,
      date,
      dayOfMonth: date.getDate(),
      inCurrentMonth: date.getMonth() === month,
      isToday: key === todayKey,
    };
  });
}

export function groupEventsByDate<T extends { starts_at: string }>(events: T[]): Record<string, T[]> {
  return events.reduce<Record<string, T[]>>((grouped, event) => {
    const key = toLocalDateKey(event.starts_at);
    grouped[key] = grouped[key] ?? [];
    grouped[key].push(event);
    return grouped;
  }, {});
}

export function toLocalDateKey(value: string | Date): string {
  const date = typeof value === "string" ? new Date(value) : value;
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function stripSlotSide(slotID: string) {
  return slotID.replace(/^(home|opponent):/, "");
}

function opponentMirrorY(sourceY: number) {
  const mirrored = clamp(100 - sourceY);
  if (mirrored > 42 && mirrored < 58) {
    return mirrored <= 50 ? 42 : 58;
  }
  return mirrored;
}

function clamp(value: number) {
  return Math.max(4, Math.min(96, Math.round(value * 10) / 10));
}
