import { DndContext, type DragEndEvent, useDraggable } from "@dnd-kit/core";
import { CSS } from "@dnd-kit/utilities";
import {
  CalendarDays,
  ClipboardCheck,
  LayoutDashboard,
  LogOut,
  Plus,
  Save,
  ShieldCheck,
  Users,
} from "lucide-react";
import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  APIClient,
  Player,
  TacticBoard,
  TacticSlot,
  TacticTemplate,
  TeamEvent,
  User,
} from "./api";
import { AttendanceRecord, AttendanceStatus, attendanceSummary, normalizeSlotPosition } from "./domain";

const tokenKey = "anypitch_token";
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

type View = "tactics" | "players" | "calendar" | "attendance";

export function App() {
  const [client] = useState(() => new APIClient(localStorage.getItem(tokenKey)));
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(tokenKey));
  const [user, setUser] = useState<User | null>(null);
  const [view, setView] = useState<View>("tactics");
  const [players, setPlayers] = useState<Player[]>([]);
  const [events, setEvents] = useState<TeamEvent[]>([]);
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
      const [me, playerResult, eventResult, templateResult, boardResult] = await Promise.all([
        client.me(),
        client.listPlayers(),
        client.listEvents(),
        client.templates(),
        client.listBoards(),
      ]);
      setUser(me.user);
      setPlayers(playerResult.players);
      setEvents(eventResult.events);
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
          <CalendarPanel client={client} events={events} onEventsChanged={setEvents} onError={setError} />
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
  const [mode, setMode] = useState<"login" | "register">("register");
  const [email, setEmail] = useState("coach@example.com");
  const [password, setPassword] = useState("correct horse battery staple");
  const [localError, setLocalError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    setLocalError("");
    try {
      const session = mode === "register" ? await client.register(email, password) : await client.login(email, password);
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
          <h1>把每一次训练和战术部署落到可执行的队伍管理里</h1>
        </div>
        <form className="auth-form" onSubmit={submit}>
          <div className="mode-switch" role="tablist" aria-label="认证方式">
            <button type="button" className={mode === "register" ? "active" : ""} onClick={() => setMode("register")}>
              注册
            </button>
            <button type="button" className={mode === "login" ? "active" : ""} onClick={() => setMode("login")}>
              登录
            </button>
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
            {mode === "register" ? "创建教练工作台" : "进入教练工作台"}
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
  const fallbackTemplate = templates[0];
  const [templateIndex, setTemplateIndex] = useState(0);
  const activeTemplate = templates[templateIndex] ?? fallbackTemplate;
  const [boardName, setBoardName] = useState("五人制高位压迫");
  const [slots, setSlots] = useState<TacticSlot[]>([]);
  const [selectedSlotID, setSelectedSlotID] = useState<string>("");
  const fieldRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (activeTemplate) {
      setBoardName(activeTemplate.name);
      setSlots(activeTemplate.slots.map((slot) => ({ ...slot, player_id: slot.player_id || "" })));
      setSelectedSlotID(activeTemplate.slots[0]?.slot_id ?? "");
    }
  }, [activeTemplate?.format]);

  async function saveBoard() {
    if (!activeTemplate) {
      return;
    }
    try {
      const result = await client.createBoard({
        name: boardName,
        format: activeTemplate.format,
        formation: activeTemplate.formation,
        slots,
      });
      onBoardsChanged([result.board, ...boards]);
      onError("");
    } catch (err) {
      onError(messageFromError(err));
    }
  }

  function onDragEnd(event: DragEndEvent) {
    const field = fieldRef.current;
    if (!field) {
      return;
    }
    const rect = field.getBoundingClientRect();
    setSlots((current) =>
      current.map((slot) => {
        if (slot.slot_id !== event.active.id) {
          return slot;
        }
        const next = normalizeSlotPosition({
          x: slot.x + (event.delta.x / rect.width) * 100,
          y: slot.y + (event.delta.y / rect.height) * 100,
        });
        return { ...slot, ...next };
      }),
    );
  }

  function assignSelectedSlot(playerID: string) {
    setSlots((current) => current.map((slot) => (slot.slot_id === selectedSlotID ? { ...slot, player_id: playerID } : slot)));
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
          人制模板
          <select value={templateIndex} onChange={(event) => setTemplateIndex(Number(event.target.value))}>
            {templates.map((template, index) => (
              <option value={index} key={template.format}>
                {template.format} 人制 · {template.formation}
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
          <select value={slots.find((slot) => slot.slot_id === selectedSlotID)?.player_id ?? ""} onChange={(event) => assignSelectedSlot(event.target.value)}>
            <option value="">未分配</option>
            {players.map((player) => (
              <option key={player.id} value={player.id}>
                {player.number ? `${player.number} · ` : ""}
                {player.name}
              </option>
            ))}
          </select>
        </label>
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
              </small>
            </button>
          ))}
        </div>
      </div>
      <DndContext onDragEnd={onDragEnd}>
        <div className="pitch-shell">
          <div className="pitch" ref={fieldRef}>
            <div className="pitch-line center-line" />
            <div className="pitch-circle" />
            <div className="box top-box" />
            <div className="box bottom-box" />
            {slots.map((slot) => (
              <DraggableSlot
                key={slot.slot_id}
                slot={slot}
                selected={selectedSlotID === slot.slot_id}
                player={players.find((player) => player.id === slot.player_id)}
                onSelect={() => setSelectedSlotID(slot.slot_id)}
              />
            ))}
          </div>
        </div>
      </DndContext>
    </section>
  );
}

function DraggableSlot({
  slot,
  selected,
  player,
  onSelect,
}: {
  slot: TacticSlot;
  selected: boolean;
  player?: Player;
  onSelect: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: slot.slot_id });
  const dragTransform = transform ? CSS.Translate.toString(transform) : "";
  return (
    <button
      ref={setNodeRef}
      className={`slot-marker ${selected ? "selected" : ""}`}
      style={{
        left: `${slot.x}%`,
        top: `${slot.y}%`,
        transform: `translate(-50%, -50%) ${dragTransform}`,
      }}
      type="button"
      onClick={onSelect}
      {...listeners}
      {...attributes}
    >
      <span>{player?.number ?? slot.label}</span>
      <small>{player?.name ?? slot.label}</small>
    </button>
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
  const [positions, setPositions] = useState("FW, AM");

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
  onEventsChanged,
  onError,
}: {
  client: APIClient;
  events: TeamEvent[];
  onEventsChanged: (events: TeamEvent[]) => void;
  onError: (message: string) => void;
}) {
  const [title, setTitle] = useState("周三控球训练");
  const [type, setType] = useState<"training" | "friendly">("training");
  const [startsAt, setStartsAt] = useState("2026-05-13T20:00");
  const [location, setLocation] = useState("东区球场");

  async function submit(event: FormEvent) {
    event.preventDefault();
    try {
      const iso = new Date(startsAt).toISOString();
      const result = await client.createEvent({
        type,
        title,
        starts_at: iso,
        location,
        opponent: type === "friendly" ? "待定对手" : "",
        notes: "",
      });
      onEventsChanged([...events, result.event]);
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
          时间
          <input value={startsAt} onChange={(event) => setStartsAt(event.target.value)} type="datetime-local" />
        </label>
        <label>
          地点
          <input value={location} onChange={(event) => setLocation(event.target.value)} />
        </label>
        <button className="primary-button" type="submit">
          <Plus size={18} />
          添加日程
        </button>
      </form>
      <div className="data-panel timeline">
        {events.map((event) => (
          <article className="event-row" key={event.id}>
            <span className={`event-type ${event.type}`}>{event.type === "training" ? "训练" : "友谊赛"}</span>
            <div>
              <strong>{event.title}</strong>
              <small>
                {formatDate(event.starts_at)} · {event.location || "未设置地点"}
              </small>
            </div>
          </article>
        ))}
        {events.length === 0 ? <div className="empty-state">还没有训练或友谊赛</div> : null}
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
      const next: Record<string, AttendanceStatus> = {};
      for (const player of players) {
        next[player.id] = "unknown";
      }
      for (const record of result.records) {
        next[record.player_id] = record.status;
      }
      setRecords(next);
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
                  {option}
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
