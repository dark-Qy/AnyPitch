import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  type DragEndEvent,
  type DragStartEvent,
  useDraggable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ClipboardCheck,
  LayoutDashboard,
  LogOut,
  Plus,
  Save,
  ShieldCheck,
  Trash2,
  Users,
} from "lucide-react";
import { type CSSProperties, FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  APIClient,
  EventLocation,
  Player,
  TacticBoard,
  TacticSlot,
  TacticTemplate,
  TeamEvent,
  User,
} from "./api";
import {
  AttendanceRecord,
  AttendanceStatus,
  attendanceSummary,
  buildAttendanceStatusMap,
  buildMonthCalendar,
  groupEventsByDate,
  homeSlotsFromTemplate,
  normalizeSlotPosition,
  opponentSlotsFromTemplate,
  pitchGeometry,
  templatesForFormat,
  toLocalDateKey,
  type TacticFormat,
} from "./domain";

const tokenKey = "anypitch_token";
const defaultLocationName = "北京邮电大学（海淀校区）";
const attendanceOptions: AttendanceStatus[] = [
  "unknown",
  "available",
  "unavailable",
  "late",
  "injured",
  "present",
  "absent",
  "excused",
];
const attendanceLabels: Record<AttendanceStatus, string> = {
  unknown: "未确认",
  available: "可参加",
  unavailable: "不可参加",
  late: "迟到",
  injured: "伤病",
  present: "已到场",
  absent: "缺席",
  excused: "请假",
};

type View = "tactics" | "players" | "calendar" | "attendance";

export function App() {
  const [client] = useState(() => new APIClient(localStorage.getItem(tokenKey)));
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(tokenKey));
  const [user, setUser] = useState<User | null>(null);
  const [view, setView] = useState<View>("tactics");
  const [players, setPlayers] = useState<Player[]>([]);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  const [locations, setLocations] = useState<EventLocation[]>([]);
  const [templates, setTemplates] = useState<TacticTemplate[]>([]);
  const [boards, setBoards] = useState<TacticBoard[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    client.setToken(token);
    if (token) {
      void bootstrap();
    }
  }, [token]);

  async function bootstrap() {
    setBusy(true);
    setError("");
    try {
      const [me, playerResult, eventResult, locationResult, templateResult, boardResult] = await Promise.all([
        client.me(),
        client.listPlayers(),
        client.listEvents(),
        client.listLocations(),
        client.templates(),
        client.listBoards(),
      ]);
      setUser(me.user);
      setPlayers(playerResult.players);
      setEvents(eventResult.events);
      setLocations(locationResult.locations);
      setTemplates(templateResult.templates);
      setBoards(boardResult.boards);
    } catch (err) {
      setError(messageFromError(err));
      setToken(null);
      localStorage.removeItem(tokenKey);
    } finally {
      setBusy(false);
    }
  }

  async function handleAuthenticated(sessionToken: string, nextUser: User) {
    localStorage.setItem(tokenKey, sessionToken);
    setToken(sessionToken);
    setUser(nextUser);
  }

  async function logout() {
    try {
      await client.logout();
    } catch {
      // Local logout remains useful when the session already expired.
    }
    localStorage.removeItem(tokenKey);
    setToken(null);
    setUser(null);
  }

  if (!token || !user) {
    return <AuthGateway client={client} busy={busy} error={error} onAuthenticated={handleAuthenticated} />;
  }

  return (
    <div className="app-shell">
      <aside className="side-rail" aria-label="主导航">
        <div className="brand-lockup">
          <div className="brand-mark">AP</div>
          <div>
            <strong>AnyPitch</strong>
            <span>Coach Workbench</span>
          </div>
        </div>
        <nav className="nav-stack">
          <NavButton active={view === "tactics"} icon={<LayoutDashboard size={18} />} label="战术板" onClick={() => setView("tactics")} />
          <NavButton active={view === "players"} icon={<Users size={18} />} label="队员" onClick={() => setView("players")} />
          <NavButton active={view === "calendar"} icon={<CalendarDays size={18} />} label="日程" onClick={() => setView("calendar")} />
          <NavButton active={view === "attendance"} icon={<ClipboardCheck size={18} />} label="出勤" onClick={() => setView("attendance")} />
        </nav>
        <button className="ghost-button rail-logout" type="button" onClick={logout} title="退出登录">
          <LogOut size={18} />
          <span>退出</span>
        </button>
      </aside>

      <main className="workspace">
        {error ? <div className="error-banner">{error}</div> : null}

        {view === "tactics" ? (
          <TacticsPanel
            client={client}
            players={players}
            templates={templates}
            boards={boards}
            onBoardsChanged={setBoards}
            onError={setError}
          />
        ) : null}
        {view === "players" ? (
          <PlayersPanel client={client} players={players} onPlayersChanged={setPlayers} onError={setError} />
        ) : null}
        {view === "calendar" ? (
          <CalendarPanel
            client={client}
            events={events}
            locations={locations}
            players={players}
            onEventsChanged={setEvents}
            onLocationsChanged={setLocations}
            onError={setError}
          />
        ) : null}
        {view === "attendance" ? (
          <AttendancePanel client={client} events={events} players={players} onError={setError} />
        ) : null}
      </main>
    </div>
  );
}

function AuthGateway({
  client,
  busy,
  error,
  onAuthenticated,
}: {
  client: APIClient;
  busy: boolean;
  error: string;
  onAuthenticated: (token: string, user: User) => void;
}) {
  const [email, setEmail] = useState("coach@anypitch.local");
  const [password, setPassword] = useState("AnyPitch@2026");
  const [localError, setLocalError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    setLocalError("");
    try {
      const session = await client.login(email, password);
      onAuthenticated(session.token, session.user);
    } catch (err) {
      setLocalError(messageFromError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-card">
        <div className="auth-brand">
          <div className="brand-mark large">AP</div>
          <p>AnyPitch</p>
          <h1>单队教练工作台</h1>
        </div>
        <form className="auth-form" onSubmit={submit}>
          <div className="login-note">
            <strong>默认教练账号</strong>
            <span>coach@anypitch.local / AnyPitch@2026</span>
          </div>
          <label>
            邮箱
            <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="email" />
          </label>
          <label>
            密码
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" />
          </label>
          {localError || error ? <div className="error-banner compact">{localError || error}</div> : null}
          <button className="primary-button" disabled={submitting || busy} type="submit">
            <ShieldCheck size={18} />
            登录
          </button>
        </form>
      </section>
    </main>
  );
}

function TacticsPanel({
  client,
  players,
  templates,
  boards,
  onBoardsChanged,
  onError,
}: {
  client: APIClient;
  players: Player[];
  templates: TacticTemplate[];
  boards: TacticBoard[];
  onBoardsChanged: (boards: TacticBoard[]) => void;
  onError: (message: string) => void;
}) {
  const [format, setFormat] = useState<TacticFormat>(5);
  const formatTemplates = useMemo(() => templatesForFormat(templates, format), [templates, format]);
  const [templateID, setTemplateID] = useState("");
  const [opponentEnabled, setOpponentEnabled] = useState(false);
  const [opponentTemplateID, setOpponentTemplateID] = useState("");
  const activeTemplate = formatTemplates.find((template) => template.id === templateID) ?? formatTemplates[0] ?? templates[0];
  const opponentTemplate = formatTemplates.find((template) => template.id === opponentTemplateID) ?? activeTemplate;
  const geometry = pitchGeometry(format);
  const [boardName, setBoardName] = useState("五人制高位压迫");
  const [slots, setSlots] = useState<TacticSlot[]>([]);
  const [selectedSlotID, setSelectedSlotID] = useState<string>("");
  const [activeDragID, setActiveDragID] = useState<string>("");
  const fieldRef = useRef<HTMLDivElement | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
    useSensor(KeyboardSensor),
  );
  const activeDragSlot = slots.find((slot) => slot.slot_id === activeDragID);
  const selectedSlot = slots.find((slot) => slot.slot_id === selectedSlotID);

  useEffect(() => {
    if (!formatTemplates.some((template) => template.id === templateID)) {
      setTemplateID(formatTemplates[0]?.id ?? "");
    }
    if (!formatTemplates.some((template) => template.id === opponentTemplateID)) {
      setOpponentTemplateID(formatTemplates[0]?.id ?? "");
    }
  }, [formatTemplates, opponentTemplateID, templateID]);

  useEffect(() => {
    if (!activeTemplate) {
      return;
    }
    const nextSlots = homeSlotsFromTemplate(activeTemplate);
    setBoardName(activeTemplate.name);
    setSlots(opponentEnabled && opponentTemplate ? [...nextSlots, ...opponentSlotsFromTemplate(opponentTemplate)] : nextSlots);
    setSelectedSlotID(nextSlots[0]?.slot_id ?? "");
  }, [activeTemplate?.id, opponentEnabled, opponentTemplate?.id]);

  async function saveBoard() {
    if (!activeTemplate) {
      return;
    }
    try {
      const result = await client.createBoard({
        name: boardName,
        template_id: activeTemplate.id,
        opponent_template_id: opponentEnabled ? opponentTemplate?.id : "",
        format: activeTemplate.format,
        formation: activeTemplate.formation,
        opponent_formation: opponentEnabled ? opponentTemplate?.formation : "",
        slots,
      });
      onBoardsChanged([result.board, ...boards]);
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  function onDragStart(event: DragStartEvent) {
    const slotID = String(event.active.id);
    setActiveDragID(slotID);
    setSelectedSlotID(slotID);
  }

  function onDragEnd(event: DragEndEvent) {
    const field = fieldRef.current;
    const translated = event.active.rect.current.translated;
    if (!field || !translated) {
      setActiveDragID("");
      return;
    }
    const rect = field.getBoundingClientRect();
    const next = normalizeSlotPosition({
      x: ((translated.left + translated.width / 2 - rect.left) / rect.width) * 100,
      y: ((translated.top + translated.height / 2 - rect.top) / rect.height) * 100,
    });
    setSlots((current) =>
      current.map((slot) => {
        if (slot.slot_id !== event.active.id) {
          return slot;
        }
        return { ...slot, ...next };
      }),
    );
    setActiveDragID("");
  }

  function assignSelectedSlot(playerID: string) {
    setSlots((current) => current.map((slot) => (slot.slot_id === selectedSlotID && slot.side !== "opponent" ? { ...slot, player_id: playerID } : slot)));
  }

  if (!activeTemplate) {
    return <div className="empty-state">战术模板加载中</div>;
  }

  return (
    <section className="panel-grid tactics-grid">
      <div className="tool-panel">
        <div className="panel-heading">
          <h2>拖拽站位</h2>
        </div>
        <label>
          赛制
          <select value={format} onChange={(event) => setFormat(Number(event.target.value) as TacticFormat)}>
            {[5, 8, 11].map((nextFormat) => (
              <option value={nextFormat} key={nextFormat}>
                {nextFormat} 人制
              </option>
            ))}
          </select>
        </label>
        <label>
          我方模板
          <select value={activeTemplate.id} onChange={(event) => setTemplateID(event.target.value)}>
            {formatTemplates.map((template) => (
              <option value={template.id} key={template.id}>
                {template.name} · {template.formation}
              </option>
            ))}
          </select>
        </label>
        <label>
          战术名称
          <input value={boardName} onChange={(event) => setBoardName(event.target.value)} />
        </label>
        <label>
          选中位置分配队员
          <select
            value={selectedSlot?.side === "opponent" ? "" : selectedSlot?.player_id ?? ""}
            disabled={selectedSlot?.side === "opponent"}
            onChange={(event) => assignSelectedSlot(event.target.value)}
          >
            <option value="">未分配</option>
            {players.map((player) => (
              <option key={player.id} value={player.id}>
                {player.number ? `${player.number} · ` : ""}
                {player.name}
              </option>
            ))}
          </select>
        </label>
        <label className="toggle-row">
          <input type="checkbox" checked={opponentEnabled} onChange={(event) => setOpponentEnabled(event.target.checked)} />
          添加对手站位
        </label>
        {opponentEnabled ? (
          <label>
            对手模板
            <select value={opponentTemplate?.id ?? ""} onChange={(event) => setOpponentTemplateID(event.target.value)}>
              {formatTemplates.map((template) => (
                <option value={template.id} key={template.id}>
                  {template.name} · {template.formation}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <button className="primary-button" type="button" onClick={saveBoard}>
          <Save size={18} />
          保存战术板
        </button>
        <div className="saved-list">
          <strong>最近保存</strong>
          {boards.length === 0 ? <span>还没有保存的战术板</span> : null}
          {boards.slice(0, 4).map((board) => (
            <button className="saved-board" type="button" key={board.id}>
              <span>{board.name}</span>
              <small>
                {board.format}v{board.format} · {board.formation}
                {board.opponent_formation ? ` vs ${board.opponent_formation}` : ""}
              </small>
            </button>
          ))}
        </div>
      </div>
      <DndContext sensors={sensors} autoScroll={false} onDragStart={onDragStart} onDragEnd={onDragEnd} onDragCancel={() => setActiveDragID("")}>
        <div className="pitch-shell">
          <div
            className={`pitch pitch-${format} ${opponentEnabled ? "has-opponent" : ""}`}
            ref={fieldRef}
            style={
              {
                "--pitch-ratio": geometry.ratio,
                "--pitch-max-width": geometry.maxWidth,
              } as CSSProperties
            }
          >
            <div className="pitch-line center-line" />
            <div className="pitch-circle" />
            <div className="box top-box" />
            <div className="box bottom-box" />
            {slots.map((slot) => (
              <DraggableSlot
                key={slot.slot_id}
                slot={slot}
                selected={selectedSlotID === slot.slot_id}
                dragging={activeDragID === slot.slot_id}
                player={slot.side === "opponent" ? undefined : players.find((player) => player.id === slot.player_id)}
                onSelect={() => setSelectedSlotID(slot.slot_id)}
              />
            ))}
          </div>
        </div>
        <DragOverlay dropAnimation={{ duration: 120, easing: "ease-out" }}>
          {activeDragSlot ? (
            <div className={`slot-marker ${activeDragSlot.side === "opponent" ? "opponent" : ""} drag-overlay`}>
              <SlotMarkerContent
                slot={activeDragSlot}
                player={activeDragSlot.side === "opponent" ? undefined : players.find((player) => player.id === activeDragSlot.player_id)}
              />
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
    </section>
  );
}

function DraggableSlot({
  slot,
  selected,
  dragging,
  player,
  onSelect,
}: {
  slot: TacticSlot;
  selected: boolean;
  dragging: boolean;
  player?: Player;
  onSelect: () => void;
}) {
  const { attributes, listeners, setNodeRef } = useDraggable({ id: slot.slot_id });
  return (
    <button
      ref={setNodeRef}
      className={`slot-marker ${slot.side === "opponent" ? "opponent" : ""} ${selected ? "selected" : ""} ${dragging ? "dragging" : ""}`}
      style={{
        left: `${slot.x}%`,
        top: `${slot.y}%`,
        transform: "translate(-50%, -50%)",
      }}
      type="button"
      onClick={onSelect}
      {...listeners}
      {...attributes}
    >
      <SlotMarkerContent slot={slot} player={player} />
    </button>
  );
}

function SlotMarkerContent({ slot, player }: { slot: TacticSlot; player?: Player }) {
  const subLabel = player?.name ?? "";

  return (
    <>
      <span>{player?.number ?? slot.label}</span>
      {subLabel ? <small>{subLabel}</small> : null}
    </>
  );
}

function PlayersPanel({
  client,
  players,
  onPlayersChanged,
  onError,
}: {
  client: APIClient;
  players: Player[];
  onPlayersChanged: (players: Player[]) => void;
  onError: (message: string) => void;
}) {
  const [name, setName] = useState("");
  const [number, setNumber] = useState("");
  const [positions, setPositions] = useState("前锋, 前腰");

  async function submit(event: FormEvent) {
    event.preventDefault();
    try {
      const result = await client.createPlayer({
        name,
        number: number ? Number(number) : null,
        positions: positions.split(",").map((value) => value.trim()),
      });
      onPlayersChanged([...players, result.player]);
      setName("");
      setNumber("");
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  return (
    <section className="panel-grid">
      <form className="tool-panel" onSubmit={submit}>
        <div className="panel-heading">
          <h2>新增队员</h2>
        </div>
        <label>
          姓名
          <input value={name} onChange={(event) => setName(event.target.value)} placeholder="例如 林海" />
        </label>
        <label>
          号码
          <input value={number} onChange={(event) => setNumber(event.target.value)} type="number" min="1" max="99" />
        </label>
        <label>
          位置
          <input value={positions} onChange={(event) => setPositions(event.target.value)} />
        </label>
        <button className="primary-button" type="submit">
          <Plus size={18} />
          添加队员
        </button>
      </form>
      <div className="data-panel">
        {players.map((player) => (
          <article className="player-row" key={player.id}>
            <strong>{player.number ?? "--"}</strong>
            <div>
              <span>{player.name}</span>
              <small>{player.positions.join(" / ") || "未设置位置"}</small>
            </div>
          </article>
        ))}
        {players.length === 0 ? <div className="empty-state">还没有队员</div> : null}
      </div>
    </section>
  );
}

function CalendarPanel({
  client,
  events,
  locations,
  players,
  onEventsChanged,
  onLocationsChanged,
  onError,
}: {
  client: APIClient;
  events: TeamEvent[];
  locations: EventLocation[];
  players: Player[];
  onEventsChanged: (events: TeamEvent[]) => void;
  onLocationsChanged: (locations: EventLocation[]) => void;
  onError: (message: string) => void;
}) {
  const today = useMemo(() => new Date(), []);
  const todayKey = toLocalDateKey(today);
  const [title, setTitle] = useState("周三控球训练");
  const [type, setType] = useState<"training" | "friendly">("training");
  const [selectedDate, setSelectedDate] = useState(todayKey);
  const [startsAt, setStartsAt] = useState(`${todayKey}T20:00`);
  const [endsAt, setEndsAt] = useState(`${todayKey}T22:00`);
  const [location, setLocation] = useState(defaultLocationName);
  const [newLocation, setNewLocation] = useState("");
  const [selectedEventID, setSelectedEventID] = useState("");
  const [records, setRecords] = useState<Record<string, AttendanceStatus>>({});
  const [monthCursor, setMonthCursor] = useState(() => new Date(today.getFullYear(), today.getMonth(), 1));
  const monthDays = useMemo(() => buildMonthCalendar(monthCursor.getFullYear(), monthCursor.getMonth(), today), [monthCursor, today]);
  const eventsByDate = useMemo(() => groupEventsByDate(events), [events]);
  const selectedEvent = events.find((event) => event.id === selectedEventID);
  const summary = useMemo(
    () =>
      attendanceSummary(
        Object.entries(records).map(([playerID, status]) => ({
          player_id: playerID,
          status,
          note: "",
          updated_at: "",
        })),
      ),
    [records],
  );
  const monthLabel = new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "long" }).format(monthCursor);

  useEffect(() => {
    if (locations.length > 0 && !locations.some((next) => next.name === location)) {
      setLocation(locations[0].name);
    }
  }, [locations, location]);

  useEffect(() => {
    if (selectedEventID && !events.some((event) => event.id === selectedEventID)) {
      setSelectedEventID("");
    }
  }, [events, selectedEventID]);

  useEffect(() => {
    if (selectedEventID) {
      void loadEventAttendance(selectedEventID);
    }
  }, [selectedEventID, players]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    try {
      const iso = new Date(startsAt).toISOString();
      const endISO = new Date(endsAt).toISOString();
      const result = await client.createEvent({
        type,
        title,
        starts_at: iso,
        ends_at: endISO,
        location,
        opponent: type === "friendly" ? "待定对手" : "",
        notes: "",
      });
      onEventsChanged([...events, result.event].sort((left, right) => left.starts_at.localeCompare(right.starts_at)));
      setSelectedEventID(result.event.id);
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  function shiftMonth(delta: number) {
    setMonthCursor((current) => new Date(current.getFullYear(), current.getMonth() + delta, 1));
  }

  function selectCalendarDate(dayKey: string) {
    setSelectedDate(dayKey);
    setStartsAt(`${dayKey}T20:00`);
    setEndsAt(`${dayKey}T22:00`);
    const next = parseDateKey(dayKey);
    setMonthCursor(new Date(next.getFullYear(), next.getMonth(), 1));
  }

  function changeStartsAt(value: string) {
    setStartsAt(value);
    if (endsAt <= value) {
      setEndsAt(addHoursToLocalInput(value, 2));
    }
    const nextDate = value.slice(0, 10);
    if (nextDate) {
      setSelectedDate(nextDate);
      const next = parseDateKey(nextDate);
      setMonthCursor(new Date(next.getFullYear(), next.getMonth(), 1));
    }
  }

  function selectCalendarEvent(event: TeamEvent) {
    const dayKey = toLocalDateKey(event.starts_at);
    setSelectedDate(dayKey);
    setStartsAt(`${dayKey}T20:00`);
    setEndsAt(`${dayKey}T22:00`);
    setSelectedEventID(event.id);
    const next = parseDateKey(dayKey);
    setMonthCursor(new Date(next.getFullYear(), next.getMonth(), 1));
  }

  async function loadEventAttendance(eventID: string) {
    try {
      const result = await client.listAttendance(eventID);
      setRecords(buildAttendanceStatusMap(players, result.records));
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  async function saveEventAttendance() {
    if (!selectedEventID) {
      return;
    }
    try {
      await client.saveAttendance(
        selectedEventID,
        players.map((player) => ({
          player_id: player.id,
          status: records[player.id] ?? "unknown",
          note: "",
        })),
      );
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  async function addLocation() {
    const name = newLocation.trim();
    if (!name) {
      return;
    }
    try {
      const result = await client.createLocation({ name });
      const next = [...locations.filter((item) => item.id !== result.location.id), result.location];
      onLocationsChanged(next);
      setLocation(result.location.name);
      setNewLocation("");
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  async function deleteSelectedLocation() {
    const selected = locations.find((item) => item.name === location);
    if (!selected || selected.name === defaultLocationName) {
      return;
    }
    try {
      await client.deleteLocation(selected.id);
      const next = locations.filter((item) => item.id !== selected.id);
      onLocationsChanged(next);
      setLocation(next[0]?.name ?? defaultLocationName);
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  return (
    <section className="panel-grid">
      <form className="tool-panel" onSubmit={submit}>
        <div className="panel-heading">
          <h2>训练 / 友谊赛</h2>
        </div>
        <label>
          类型
          <select value={type} onChange={(event) => setType(event.target.value as "training" | "friendly")}>
            <option value="training">训练</option>
            <option value="friendly">友谊赛</option>
          </select>
        </label>
        <label>
          标题
          <input value={title} onChange={(event) => setTitle(event.target.value)} />
        </label>
        <label>
          开始时间
          <input value={startsAt} onChange={(event) => changeStartsAt(event.target.value)} type="datetime-local" />
        </label>
        <label>
          结束时间
          <input value={endsAt} onChange={(event) => setEndsAt(event.target.value)} type="datetime-local" />
        </label>
        <label>
          地点
          <select value={location} onChange={(event) => setLocation(event.target.value)}>
            {(locations.length > 0 ? locations : [{ id: "default", name: defaultLocationName } as EventLocation]).map((item) => (
              <option value={item.name} key={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </label>
        <div className="location-manager">
          <input value={newLocation} onChange={(event) => setNewLocation(event.target.value)} placeholder="新增地点" />
          <button className="ghost-button icon-button" type="button" onClick={addLocation} aria-label="新增地点">
            <Plus size={17} />
          </button>
          <button
            className="ghost-button icon-button"
            type="button"
            onClick={deleteSelectedLocation}
            disabled={!locations.some((item) => item.name === location && item.name !== defaultLocationName)}
            aria-label="删除当前地点"
          >
            <Trash2 size={17} />
          </button>
        </div>
        <button className="primary-button" type="submit">
          <Plus size={18} />
          添加到 {selectedDate.slice(5)}
        </button>
      </form>
      <div className="data-panel calendar-board">
        <div className="calendar-toolbar">
          <button className="ghost-button icon-button" type="button" onClick={() => shiftMonth(-1)} aria-label="上个月">
            <ChevronLeft size={18} />
          </button>
          <strong>{monthLabel}</strong>
          <button className="ghost-button icon-button" type="button" onClick={() => shiftMonth(1)} aria-label="下个月">
            <ChevronRight size={18} />
          </button>
        </div>
        <div className="weekday-row" aria-hidden="true">
          {["一", "二", "三", "四", "五", "六", "日"].map((weekday) => (
            <span key={weekday}>{weekday}</span>
          ))}
        </div>
        <div className="month-grid">
          {monthDays.map((day) => {
            const dayEvents = eventsByDate[day.key] ?? [];
            return (
              <div
                className={`day-cell ${day.inCurrentMonth ? "" : "muted"} ${day.key === selectedDate ? "selected" : ""} ${
                  day.isToday ? "today" : ""
                }`}
                key={day.key}
              >
                <button className="day-pick" type="button" onClick={() => selectCalendarDate(day.key)}>
                  <span className="day-number">{day.dayOfMonth}</span>
                </button>
                <span className="day-events">
                  {dayEvents.slice(0, 2).map((event) => (
                    <button
                      className={`event-chip ${event.type} ${selectedEventID === event.id ? "active" : ""}`}
                      type="button"
                      key={event.id}
                      onClick={() => selectCalendarEvent(event)}
                    >
                      {event.type === "training" ? "训" : "赛"} {event.title}
                    </button>
                  ))}
                  {dayEvents.length > 2 ? <small className="event-chip more">+{dayEvents.length - 2}</small> : null}
                </span>
              </div>
            );
          })}
        </div>
        {events.length === 0 ? <div className="empty-state">点击日期，新增第一条训练或友谊赛</div> : null}
        <div className="event-detail-panel">
          {selectedEvent ? (
            <>
              <div className="event-detail-head">
                <div>
                  <strong>{selectedEvent.title}</strong>
                  <small>
                    {formatDateRange(selectedEvent.starts_at, selectedEvent.ends_at)} · {selectedEvent.location}
                  </small>
                </div>
                <span className={`event-type ${selectedEvent.type}`}>{selectedEvent.type === "training" ? "训练" : "友谊赛"}</span>
              </div>
              <div className="summary-box">
                <span>可用 {summary.ready}</span>
                <span>不可用 {summary.blocked}</span>
              </div>
              <div className="inline-attendance-list">
                {players.map((player) => (
                  <article className="attendance-row compact" key={player.id}>
                    <div>
                      <strong>{player.name}</strong>
                      <small>{player.positions.join(" / ") || "未设置位置"}</small>
                    </div>
                    <select
                      value={records[player.id] ?? "unknown"}
                      onChange={(event) =>
                        setRecords((current) => ({
                          ...current,
                          [player.id]: event.target.value as AttendanceStatus,
                        }))
                      }
                    >
                      {attendanceOptions.map((option) => (
                        <option value={option} key={option}>
                          {attendanceLabels[option]}
                        </option>
                      ))}
                    </select>
                  </article>
                ))}
              </div>
              <button className="primary-button" type="button" onClick={saveEventAttendance}>
                <Save size={18} />
                保存本日程出勤
              </button>
              {players.length === 0 ? <div className="empty-state">先添加队员，再管理这个日程的出勤</div> : null}
            </>
          ) : (
            <div className="empty-state">点击月历中的具体日程，直接管理队员出勤</div>
          )}
        </div>
      </div>
    </section>
  );
}

function AttendancePanel({
  client,
  events,
  players,
  onError,
}: {
  client: APIClient;
  events: TeamEvent[];
  players: Player[];
  onError: (message: string) => void;
}) {
  const [eventID, setEventID] = useState("");
  const [records, setRecords] = useState<Record<string, AttendanceStatus>>({});
  const selectedEventID = eventID || events[0]?.id || "";
  const summary = useMemo(
    () =>
      attendanceSummary(
        Object.entries(records).map(([playerID, status]) => ({
          player_id: playerID,
          status,
          note: "",
          updated_at: "",
        })),
      ),
    [records],
  );

  useEffect(() => {
    if (selectedEventID) {
      void loadAttendance(selectedEventID);
    }
  }, [selectedEventID]);

  async function loadAttendance(nextEventID: string) {
    try {
      const result = await client.listAttendance(nextEventID);
      setRecords(buildAttendanceStatusMap(players, result.records));
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  async function save() {
    if (!selectedEventID) {
      return;
    }
    try {
      await client.saveAttendance(
        selectedEventID,
        players.map((player) => ({
          player_id: player.id,
          status: records[player.id] ?? "unknown",
          note: "",
        })),
      );
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  return (
    <section className="panel-grid">
      <div className="tool-panel">
        <div className="panel-heading">
          <h2>出勤登记</h2>
        </div>
        <label>
          选择日程
          <select
            value={selectedEventID}
            onChange={(event) => {
              setEventID(event.target.value);
            }}
          >
            {events.map((event) => (
              <option value={event.id} key={event.id}>
                {event.title}
              </option>
            ))}
          </select>
        </label>
        <div className="summary-box">
          <span>可用 {summary.ready}</span>
          <span>不可用 {summary.blocked}</span>
        </div>
        <button className="primary-button" type="button" onClick={save}>
          <Save size={18} />
          保存出勤
        </button>
      </div>
      <div className="data-panel attendance-list">
        {players.map((player) => (
          <article className="attendance-row" key={player.id}>
            <div>
              <strong>{player.name}</strong>
              <small>{player.positions.join(" / ") || "未设置位置"}</small>
            </div>
            <select
              value={records[player.id] ?? "unknown"}
              onChange={(event) =>
                setRecords((current) => ({
                  ...current,
                  [player.id]: event.target.value as AttendanceStatus,
                }))
              }
            >
              {attendanceOptions.map((option) => (
                <option value={option} key={option}>
                  {attendanceLabels[option]}
                </option>
              ))}
            </select>
          </article>
        ))}
        {players.length === 0 || events.length === 0 ? <div className="empty-state">先添加队员和日程，再登记出勤</div> : null}
      </div>
    </section>
  );
}

function NavButton({ active, icon, label, onClick }: { active: boolean; icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button className={`nav-button ${active ? "active" : ""}`} type="button" onClick={onClick} aria-label={label}>
      {icon}
      <span>{label}</span>
    </button>
  );
}

function messageFromError(error: unknown) {
  return error instanceof Error ? error.message : "请求失败";
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function formatDateRange(startValue: string, endValue: string) {
  const start = new Date(startValue);
  const end = new Date(endValue);
  const date = new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
  }).format(start);
  const time = new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
  return `${date} ${time.format(start)}-${time.format(end)}`;
}

function parseDateKey(dateKey: string) {
  const [year, month, day] = dateKey.split("-").map(Number);
  return new Date(year, month - 1, day);
}

function addHoursToLocalInput(value: string, hours: number) {
  const date = new Date(value);
  date.setHours(date.getHours() + hours);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}`;
}
