export type AttendanceStatus =
  | "unknown"
  | "available"
  | "unavailable"
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

export type SlotPosition = {
  x: number;
  y: number;
};

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

export function normalizeSlotPosition(position: SlotPosition): SlotPosition {
  return {
    x: clamp(position.x),
    y: clamp(position.y),
  };
}

function clamp(value: number) {
  return Math.max(0, Math.min(100, Math.round(value * 10) / 10));
}
