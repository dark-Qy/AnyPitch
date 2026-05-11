import type { AttendanceRecord, AttendanceStatus } from "./domain";

export type User = {
  id: string;
  email: string;
  status: string;
  created_at: string;
};

export type Session = {
  token: string;
  user: User;
};

export type Player = {
  id: string;
  name: string;
  number: number | null;
  positions: string[];
  status: "active" | "inactive";
  created_at: string;
  updated_at: string;
};

export type TeamEvent = {
  id: string;
  type: "training" | "friendly";
  title: string;
  starts_at: string;
  location: string;
  opponent: string;
  notes: string;
  created_at: string;
  updated_at: string;
};

export type TacticSlot = {
  slot_id: string;
  label: string;
  side?: "home" | "opponent";
  x: number;
  y: number;
  player_id: string;
};

export type TacticTemplate = {
  id: string;
  format: 5 | 8 | 11;
  name: string;
  formation: string;
  slots: TacticSlot[];
};

export type TacticBoard = {
  id: string;
  name: string;
  template_id?: string;
  opponent_template_id?: string;
  format: 5 | 8 | 11;
  formation: string;
  opponent_formation?: string;
  slots: TacticSlot[];
  created_at: string;
  updated_at: string;
};

type APIEnvelope<T> = {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
};

export class APIClient {
  constructor(private token: string | null) {}

  setToken(token: string | null) {
    this.token = token;
  }

  login(email: string, password: string) {
    return this.request<Session>("/api/auth/login", {
      method: "POST",
      body: { email, password },
    });
  }

  logout() {
    return this.request<{ ok: boolean }>("/api/auth/logout", { method: "POST" });
  }

  me() {
    return this.request<{ user: User; team_id: string }>("/api/auth/me");
  }

  listPlayers() {
    return this.request<{ players: Player[] }>("/api/players");
  }

  createPlayer(input: { name: string; number: number | null; positions: string[] }) {
    return this.request<{ player: Player }>("/api/players", {
      method: "POST",
      body: input,
    });
  }

  listEvents() {
    return this.request<{ events: TeamEvent[] }>("/api/events");
  }

  createEvent(input: {
    type: "training" | "friendly";
    title: string;
    starts_at: string;
    location: string;
    opponent: string;
    notes: string;
  }) {
    return this.request<{ event: TeamEvent }>("/api/events", {
      method: "POST",
      body: input,
    });
  }

  listAttendance(eventID: string) {
    return this.request<{ records: AttendanceRecord[] }>(`/api/events/${eventID}/attendance`);
  }

  saveAttendance(eventID: string, records: { player_id: string; status: AttendanceStatus; note: string }[]) {
    return this.request<{ records: AttendanceRecord[] }>(`/api/events/${eventID}/attendance`, {
      method: "PUT",
      body: { records },
    });
  }

  templates() {
    return this.request<{ templates: TacticTemplate[] }>("/api/tactics/templates");
  }

  listBoards() {
    return this.request<{ boards: TacticBoard[] }>("/api/tactics/boards");
  }

  createBoard(input: {
    name: string;
    template_id?: string;
    opponent_template_id?: string;
    format: 5 | 8 | 11;
    formation: string;
    opponent_formation?: string;
    slots: TacticSlot[];
  }) {
    return this.request<{ board: TacticBoard }>("/api/tactics/boards", {
      method: "POST",
      body: input,
    });
  }

  private async request<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (this.token) {
      headers.Authorization = `Bearer ${this.token}`;
    }
    const response = await fetch(path, {
      method: init.method ?? "GET",
      headers,
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
    });
    const envelope = (await response.json()) as APIEnvelope<T>;
    if (!response.ok || envelope.error) {
      throw new Error(envelope.error?.message ?? `Request failed: ${response.status}`);
    }
    if (envelope.data === undefined) {
      throw new Error("Malformed API response");
    }
    return envelope.data;
  }
}
