import { PitchMarkings } from "./PitchMarkings";
import { ActivityTabs, AlbumPreview, EventBanner, EventContent, RankingPreview, TacticPreview, TeamCrest, TopicPreview, WeekSchedule } from "./HomeOverview";
import {
  DndContext,
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
  Check,
  ChevronRight,
  ClipboardCheck,
  Crown,
  Image as ImageIcon,
  LayoutDashboard,
  LogOut,
  MapPin,
  Menu,
  Pencil,
  Plus,
  Save,
  ShieldCheck,
  Swords,
  Trash2,
  Users,
  X,
} from "lucide-react";
import { type CSSProperties, type FormEvent, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  APIClient,
  type Album,
  type EventLocation,
  type EventType,
  type HistoryRow,
  type Leaderboard,
  type Photo,
  type Player,
  type SpecialActivity,
  type SpecialActivityEntry,
  type TacticBoard,
  type TacticSlot,
  type TacticTemplate,
  type TeamEvent,
  type Term,
  type User,
} from "./api";
import {
  type AttendanceDecisionSummary,
  type AttendanceRecord,
  type AttendanceStatus,
  type PlayerEventAttendanceRecord,
  activeRosterPlayers,
  attendanceDecisionSummary,
  calendarEventStatusClass,
  calendarEventStatusLabel,
  homeSlotsFromTemplate,
  opponentSlotsFromTemplate,
  pitchGeometry,
  positionAfterDragDelta,
  splitEventsByTime,
  templatesForFormat,
  toLocalDateTimeInput,
  twoHoursAfter,
  type TacticFormat,
} from "./domain";

const tokenKey = "anypitch_token";
const playerTokenKey = "anypitch_player_token";
const teamName = "西土城FC";
const playerStatusOptions: AttendanceStatus[] = ["available", "unavailable", "tentative", "unknown"];
const attendanceLabels: Record<AttendanceStatus, string> = { available: "参加", unavailable: "不参加", tentative: "待定", unknown: "未确认" };
const activityTypes: Array<{ value: EventType; label: string; hint: string }> = [
  { value: "training", label: "训练", hint: "日常训练，报名参加即计入训练榜" },
  { value: "friendly", label: "友谊赛", hint: "队伍约赛，参加计入训练榜" },
  { value: "pickup", label: "野球", hint: "临时约球，不计入训练榜" },
];

type View = "activities" | "tactics" | "players" | "stats" | "albums" | "special";
type SessionMode = "coach" | "player";
type AlbumTarget = { event_id?: string; special_activity_id?: string } | null;

export function App() {
  const initialPlayerToken = localStorage.getItem(playerTokenKey);
  const initialCoachToken = localStorage.getItem(tokenKey);
  const [client] = useState(() => new APIClient(initialPlayerToken ?? initialCoachToken));
  const [token, setToken] = useState<string | null>(() => initialPlayerToken ?? initialCoachToken);
  const [mode, setMode] = useState<SessionMode | null>(() => (initialPlayerToken ? "player" : initialCoachToken ? "coach" : null));
  const [view, setView] = useState<View>("activities");
  const [menuOpen, setMenuOpen] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [currentPlayer, setCurrentPlayer] = useState<Player | null>(null);
  const [players, setPlayers] = useState<Player[]>([]);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  const [playerAttendanceRecords, setPlayerAttendanceRecords] = useState<PlayerEventAttendanceRecord[]>([]);
  const [playerAttendanceSummary, setPlayerAttendanceSummary] = useState<AttendanceDecisionSummary>({ available: 0, unavailable: 0, tentative: 0, unknown: 0 });
  const [locations, setLocations] = useState<EventLocation[]>([]);
  const [templates, setTemplates] = useState<TacticTemplate[]>([]);
  const [boards, setBoards] = useState<TacticBoard[]>([]);
  const [albumTarget, setAlbumTarget] = useState<AlbumTarget>(null);
  const [busy, setBusy] = useState(Boolean(token));
  const [error, setError] = useState("");

  const bootstrapCoach = useCallback(async () => {
    setBusy(true);
    try {
      const [me, playerResult, eventResult, locationResult, templateResult, boardResult] = await Promise.all([
        client.me(), client.listPlayers(), client.listEvents(), client.listLocations(), client.templates(), client.listBoards(),
      ]);
      setUser(me.user); setCurrentPlayer(null); setPlayers(playerResult.players); setEvents(eventResult.events); setLocations(locationResult.locations); setTemplates(templateResult.templates); setBoards(boardResult.boards); setError("");
    } catch (err) { expireSession(messageFromError(err)); }
    finally { setBusy(false); }
  }, [client]);

  const bootstrapPlayer = useCallback(async () => {
    setBusy(true);
    try {
      const [me, eventResult] = await Promise.all([client.playerMe(), client.listPlayerEvents()]);
      setCurrentPlayer(me.player); setUser(null); setEvents(eventResult.events); setPlayerAttendanceRecords(eventResult.attendance_records); setPlayerAttendanceSummary(eventResult.attendance_summary); setError("");
    } catch (err) { expireSession(messageFromError(err)); }
    finally { setBusy(false); }
  }, [client]);

  useEffect(() => {
    client.setToken(token);
    if (!token || !mode) return;
    void (mode === "coach" ? bootstrapCoach() : bootstrapPlayer());
  }, [bootstrapCoach, bootstrapPlayer, client, mode, token]);

  function expireSession(message = "") {
    setToken(null); setMode(null); setUser(null); setCurrentPlayer(null); localStorage.removeItem(tokenKey); localStorage.removeItem(playerTokenKey); setError(message);
  }

  async function signIn(role: SessionMode, value: string) {
    setBusy(true); setError("");
    try {
      const session = await client.login(value);
      const nextToken = session.token;
      client.setToken(nextToken);
      localStorage.removeItem(role === "coach" ? playerTokenKey : tokenKey);
      localStorage.setItem(role === "coach" ? tokenKey : playerTokenKey, nextToken);
      setMode(role); setToken(nextToken); setView("activities");
    } catch (err) { setError(messageFromError(err)); setBusy(false); }
  }

  async function signOut() {
    try { if (mode === "coach") await client.logout(); else await client.playerLogout(); } catch { /* local session must still be cleared */ }
    expireSession();
  }

  const refreshCoach = useCallback(async () => {
    const [playerResult, eventResult, locationResult, boardResult] = await Promise.all([client.listPlayers(), client.listEvents(), client.listLocations(), client.listBoards()]);
    setPlayers(playerResult.players); setEvents(eventResult.events); setLocations(locationResult.locations); setBoards(boardResult.boards);
  }, [client]);
  const refreshPlayerEvents = useCallback(async () => {
    const result = await client.listPlayerEvents(); setEvents(result.events); setPlayerAttendanceRecords(result.attendance_records); setPlayerAttendanceSummary(result.attendance_summary);
  }, [client]);

  if (!mode || !token) return <AuthGateway busy={busy} error={error} onSignIn={signIn} />;
  const coach = mode === "coach";
  const openAlbum = (target: AlbumTarget) => { setAlbumTarget(target); setView("albums"); };
  const navigation: Array<{ id: View; label: string; icon: ReactNode }> = coach
    ? [
      { id: "activities", label: "活动", icon: <LayoutDashboard /> }, { id: "tactics", label: "战术板", icon: <Swords /> },
      { id: "players", label: "队员", icon: <Users /> }, { id: "stats", label: "数据统计", icon: <ClipboardCheck /> },
      { id: "albums", label: "球队相册", icon: <ImageIcon /> }, { id: "special", label: "赛事专题", icon: <Crown /> },
    ]
    : [
      { id: "activities", label: "活动", icon: <LayoutDashboard /> }, { id: "stats", label: "我的数据", icon: <ClipboardCheck /> },
      { id: "albums", label: "球队相册", icon: <ImageIcon /> }, { id: "special", label: "赛事专题", icon: <Crown /> },
    ];

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <button className="brand" onClick={() => setView("activities")} aria-label="返回活动首页"><TeamCrest /><b>{teamName}</b></button>
        <div className="team-label"><small>{coach ? "教练工作台" : currentPlayer?.name ?? "队员入口"}</small></div>
        <nav>{navigation.map((item) => <NavButton key={item.id} active={view === item.id} icon={item.icon} label={item.label} onClick={() => setView(item.id)} />)}</nav>
        <button className="logout-button" onClick={signOut}><LogOut size={17} />退出</button>
      </aside>
      <main className="workspace">
        <header className="mobile-header"><button className="brand compact" onClick={() => setView("activities")}><TeamCrest /><b>{teamName}</b></button><button className="icon-button" onClick={() => setMenuOpen(!menuOpen)} aria-label="打开菜单" aria-expanded={menuOpen}><Menu size={20} /></button></header>
        {menuOpen ? <nav className="mobile-menu">{navigation.map(item => <NavButton key={item.id} active={view===item.id} icon={item.icon} label={item.label} onClick={()=>{setView(item.id);setMenuOpen(false);}}/>)}<button className="logout-button" onClick={signOut}><LogOut size={16}/>退出登录</button></nav> : null}
        <nav className="mobile-nav"><NavButton active={view==='activities'} icon={<CalendarDays/>} label="活动" onClick={()=>setView('activities')}/><NavButton active={view==='stats'} icon={<ClipboardCheck/>} label="训练榜" onClick={()=>setView('stats')}/><NavButton active={view==='albums'} icon={<ImageIcon/>} label="相册" onClick={()=>setView('albums')}/><NavButton active={menuOpen} icon={<Users/>} label="我的" onClick={()=>setMenuOpen(!menuOpen)}/></nav>
        {error ? <div className="notice error" role="alert"><X size={18} />{error}<button onClick={() => setError("")} aria-label="关闭提示"><X size={16} /></button></div> : null}
        {busy ? <div className="loading-line">正在同步球队数据…</div> : null}
        {coach && view === "activities" ? <CoachActivities boards={boards} onNavigate={setView} client={client} players={players} events={events} locations={locations} onEvents={setEvents} onLocations={setLocations} onOpenAlbum={openAlbum} onError={setError} /> : null}
        {!coach && view === "activities" && currentPlayer ? <PlayerActivities onNavigate={setView} client={client} player={currentPlayer} events={events} records={playerAttendanceRecords} summary={playerAttendanceSummary} onRefresh={refreshPlayerEvents} onOpenAlbum={openAlbum} onError={setError} /> : null}
        {coach && view === "tactics" ? <TacticsPanel client={client} players={activeRosterPlayers(players)} templates={templates} boards={boards} onBoardsChanged={setBoards} onError={setError} /> : null}
        {coach && view === "players" ? <PlayersPanel client={client} players={players} locations={locations} onPlayersChanged={setPlayers} onLocationsChanged={setLocations} onError={setError} /> : null}
        {view === "stats" ? <StatsPanel client={client} coach={coach} player={currentPlayer} players={players} onError={setError} /> : null}
        {view === "special" ? <SpecialActivitiesPanel client={client} coach={coach} onOpenAlbum={openAlbum} onError={setError} /> : null}
        {view === "albums" ? <AlbumsPanel client={client} playerMode={!coach} target={albumTarget} onTargetOpened={() => setAlbumTarget(null)} onError={setError} /> : null}
      </main>
    </div>
  );
}

function AuthGateway({ busy, error, onSignIn }: { busy: boolean; error: string; onSignIn: (role: SessionMode, value: string) => void }) {
  const [value,setValue]=useState("");
  function submit(event:FormEvent){event.preventDefault();onSignIn("coach",value);}
  return <div className="auth-page"><div className="auth-card"><section className="auth-hero"><span className="eyebrow">西土城 FC</span><div className="hero-ball"/><div><TeamCrest/><h1>下一场，<br/>一起上场。</h1><p>队员请使用「吸土城FC」微信小程序登录、报名和查看训练榜。</p></div></section><form className="auth-form" onSubmit={submit}><div><span className="eyebrow">COACH WORKSPACE</span><h2>网页教练工作台</h2><p>活动、专题、照片与战术由网页管理端维护。</p></div><label>教练密码<input autoFocus type="password" value={value} onChange={e=>setValue(e.target.value)} placeholder="输入密码"/></label>{error?<div className="form-error">{error}</div>:null}<button className="primary-button" disabled={busy||!value.trim()}>{busy?"正在进入…":"进入工作台"}</button></form></div></div>;
}

function CoachActivities({ boards, onNavigate, client, players, events, locations, onEvents, onLocations, onOpenAlbum, onError }: { boards: TacticBoard[]; onNavigate: (view: View) => void; client: APIClient; players: Player[]; events: TeamEvent[]; locations: EventLocation[]; onEvents: (events: TeamEvent[]) => void; onLocations: (locations: EventLocation[]) => void; onOpenAlbum: (target: AlbumTarget) => void; onError: (message: string) => void }) {
  const [typeFilter, setTypeFilter] = useState<EventType>("training");
  const [selectedID, setSelectedID] = useState("");
  const [records, setRecords] = useState<AttendanceRecord[]>([]);
  const [loadedEventID, setLoadedEventID] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [query, setQuery] = useState("");
  const [saving, setSaving] = useState<Record<string, string>>({});
  const rowQueue = useRef(new Map<string, Promise<void>>());
  const rowVersion = useRef<Record<string, number>>({});
  const activePlayers = useMemo(() => activeRosterPlayers(players), [players]);
  const filteredEvents = useMemo(() => events.filter((event) => event.type === typeFilter).sort(sortEvents), [events, typeFilter]);
  const selected = filteredEvents.find((event) => event.id === selectedID) ?? nextEvent(filteredEvents) ?? filteredEvents[filteredEvents.length - 1];
  const activeEventID = useRef(selected?.id); activeEventID.current = selected?.id;
  const [statusFilter, setStatusFilter] = useState<AttendanceStatus | "all">("all");
  const searchedPlayers = activePlayers.filter((player) => `${player.name} ${player.number ?? ""} ${player.positions.join(" ")}`.toLowerCase().includes(query.toLowerCase()));
  const statusByPlayer = useMemo(() => Object.fromEntries(records.map((record) => [record.player_id, record])), [records]);
  const summary = attendanceDecisionSummary(records, activePlayers.length);
  const visiblePlayers = searchedPlayers.filter(player => statusFilter === "all" || (statusByPlayer[player.id]?.status ?? "unknown") === statusFilter);
  const now = new Date();

  useEffect(() => { if (selected && selected.id !== selectedID) setSelectedID(selected.id); }, [selected, selectedID]);
  useEffect(() => {
    let live = true;
    setRecords([]); setLoadedEventID('');
    if (selected) {
      const eventID = selected.id;
      client.listAttendance(eventID).then(result => {
        if (live) { setRecords(result.records); setLoadedEventID(eventID); }
      }).catch(err => { if (live) onError(messageFromError(err)); });
    }
    return () => { live = false; };
  }, [client, selected?.id, onError]);
  function updateEvent(next: TeamEvent) { onEvents(events.map((event) => event.id === next.id ? next : event)); }
  function persistRSVP(playerID: string, status: AttendanceStatus, note: string) {
    if (!selected || selected.cancelled_at) return;
    const key = `${selected.id}:${playerID}`; const version = (rowVersion.current[key] ?? 0) + 1; rowVersion.current[key] = version;
    setRecords((current) => upsertRecord(current, { player_id: playerID, status, note, updated_at: new Date().toISOString() })); setSaving((current) => ({ ...current, [key]: "保存中" }));
    const job = (rowQueue.current.get(key) ?? Promise.resolve()).catch(() => undefined).then(async () => {
      const result = await client.saveAttendance(selected.id, { player_id: playerID, status, note });
      if (rowVersion.current[key] === version && activeEventID.current === selected.id) { setRecords((current) => upsertRecord(current, result.records[0])); setSaving((current) => ({ ...current, [key]: "已保存" })); }
    });
    rowQueue.current.set(key, job); void job.catch((err) => { if (rowVersion.current[key] === version) { setSaving((current) => ({ ...current, [key]: "保存失败" })); onError(messageFromError(err)); } });
  }
  async function toggleCancel() { if (!selected) return; try { const result = selected.cancelled_at ? await client.restoreEvent(selected.id) : await client.cancelEvent(selected.id); updateEvent(result.event); } catch (err) { onError(messageFromError(err)); } }
  async function deleteEvent() { if (!selected || !window.confirm("删除这条活动？有报名或相册记录的活动应使用取消。")) return; try { await client.deleteEvent(selected.id); onEvents(events.filter((event) => event.id !== selected.id)); setSelectedID(""); } catch (err) { onError(messageFromError(err)); } }
  async function addLocation(name: string) { try { const result = await client.createLocation({ name }); onLocations([...locations, result.location]); } catch (err) { onError(messageFromError(err)); } }

  return <section className="page coach-home"><PageHeader eyebrow="" title="活动" action={<button className="primary-button" onClick={() => setShowCreate(value => !value)}><Plus size={18} />新建活动</button>} />
    <ActivityTabs value={typeFilter} onChange={type=>{setTypeFilter(type);setSelectedID('');}} />
    {showCreate ? <EventForm locations={locations} initialType={typeFilter} onAddLocation={addLocation} onCreated={event=>{onEvents([...events,event]);setTypeFilter(event.type);setSelectedID(event.id);setShowCreate(false);}} client={client} onError={onError}/> : null}
    <div className="home-grid"><div className="home-main"><EventBanner event={nextEvent(filteredEvents)} />
    <WeekSchedule events={filteredEvents} selected={selected} onSelect={setSelectedID} />
    <section className="overview-card attendance-card">{selected ? <>
      <div className="overview-heading"><h3>报名情况 <span>（{new Date(selected.starts_at).toLocaleDateString('zh-CN',{month:'2-digit',day:'2-digit',weekday:'short'})}）</span></h3><input aria-label="搜索队员" value={query} onChange={event=>setQuery(event.target.value)} placeholder="搜索队员" /></div>
      <div className="attendance-filters"><button className={statusFilter==='all'?'active':''} onClick={()=>setStatusFilter('all')}>全部 {activePlayers.length}</button>{playerStatusOptions.map(status=><button key={status} className={statusFilter===status?'active':''} onClick={()=>setStatusFilter(status)}>{attendanceLabels[status]} {summary[status]}</button>)}<button className="history-link" onClick={()=>onNavigate('stats')}><ClipboardCheck size={15}/>历史出勤</button></div>
      <div className="attendance-table"><div className="attendance-table-head"><span>#</span><span>队员</span><span>选择</span><span>状态</span></div>{visiblePlayers.map(player=>{const record=statusByPlayer[player.id]??{player_id:player.id,status:'unknown' as const,note:'',updated_at:''};const key=`${selected.id}:${player.id}`;return <article className="attendance-table-row" key={player.id}><span className="jersey-number">{player.number??'—'}</span><span className="attendance-player">{player.name}</span><div className="inline-status" role="group" aria-label={`${player.name} 的报名状态`}>{playerStatusOptions.map(status=><button aria-pressed={record.status===status} aria-label={`${player.name} ${attendanceLabels[status]}`} disabled={Boolean(selected.cancelled_at) || loadedEventID !== selected.id} key={status} className={`choice-${status} ${record.status===status?'active':''}`} onClick={()=>persistRSVP(player.id,status,record.note)}><span className="choice-icon">{record.status===status?<Check size={10}/>:null}</span>{attendanceLabels[status]}</button>)}</div><small className={`row-feedback ${saving[key]==='保存失败'?'failed':''}`} role="status">{saving[key]??(record.updated_at?'已保存':'未填写')}</small></article>;})}{!visiblePlayers.length?<Empty title="没有匹配的队员" copy="调整筛选条件后再试。"/>:null}</div><p className="autosave-note">更改后自动保存</p>
      <details className="event-management"><summary>活动信息与报名管理 · {selected.title}</summary>
      <EventEditor key={selected.id} event={selected} client={client} onUpdated={updateEvent} onError={onError}/>
      <details className="event-editor"><summary>报名备注</summary>{activePlayers.map(player => { const record = statusByPlayer[player.id] ?? { player_id: player.id, status: 'unknown' as const, note: '', updated_at: '' }; return <label className="rsvp-note" key={player.id}>{player.name}<input aria-label={`${player.name} 的备注`} value={record.note} disabled={Boolean(selected.cancelled_at) || loadedEventID !== selected.id} placeholder="例如伤病、集合时间" onChange={event => setRecords(current => upsertRecord(current, { ...record, note: event.target.value }))} onBlur={event => { if(event.target.value.trim() || record.updated_at) persistRSVP(player.id,record.status,event.target.value); }} /><small role="status">{saving[`${selected.id}:${player.id}`]}</small></label>; })}</details>
      <div className="detail-actions"><button className="secondary-button" onClick={()=>onOpenAlbum({event_id:selected.id})}>本场相册</button><button className="secondary-button" onClick={toggleCancel}>{selected.cancelled_at?'恢复活动':'取消活动'}</button><button className="text-danger" onClick={deleteEvent}>删除</button></div>
      {selected.cancelled_at ? <div className="notice muted">这场活动已取消。历史报名保留，不再接受报名或计入统计。</div> : null}
      </details>
    </> : <Empty title="这个分类还没有活动" copy="新建一场活动，开始管理报名。" />}</section>
    <details className="event-archive"><summary>查看全部活动（{filteredEvents.length}）</summary><div className="event-list">{filteredEvents.map(event=><button key={event.id} className={`event-list-row ${selected?.id===event.id?'selected':''}`} onClick={()=>setSelectedID(event.id)}><span className={`type-dot ${event.type}`}/><div><strong>{event.title}</strong><small>{formatDateRange(event)}</small></div>{event.cancelled_at?'已取消':<ChevronRight size={16}/>}</button>)}</div></details>
    </div><aside className="home-aside"><TacticPreview board={boards[0]} onOpen={()=>onNavigate('tactics')}/><RankingPreview client={client} onOpen={()=>onNavigate('stats')}/></aside><AlbumPreview client={client} onOpen={album=>onOpenAlbum(album?{event_id:album.event_id,special_activity_id:album.special_activity_id}:selected?{event_id:selected.id}:null)}/></div>
  </section>;
}

function EventForm({ client, initialType, onCreated, onError }: { client: APIClient; locations: EventLocation[]; initialType: EventType; onAddLocation: (name: string) => void; onCreated: (event: TeamEvent) => void; onError: (message: string) => void }) {
  const [type, setType] = useState<EventType>(initialType); const [title, setTitle] = useState(""); const [startsAt, setStartsAt] = useState(defaultStart()); const [endsAt, setEndsAt] = useState(() => twoHoursAfter(startsAt)); const [location, setLocation] = useState("北京邮电大学（海淀校区）"); const [format, setFormat] = useState(5); const [opponent, setOpponent] = useState(""); const [notes, setNotes] = useState(""); const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent) { event.preventDefault(); setBusy(true); try { const result = await client.createEvent({ type, format: type === "friendly" ? format : 0, title: title.trim() || eventTypeLabel(type), starts_at: new Date(startsAt).toISOString(), ends_at: new Date(endsAt).toISOString(), location, opponent, notes }); onCreated(result.event); } catch (err) { onError(messageFromError(err)); } finally { setBusy(false); } }
  return <form className="event-form" onSubmit={submit}><div className="section-heading"><div><h3>新建活动</h3></div></div><div className="form-grid"><label>类型<select value={type} onChange={(event) => setType(event.target.value as EventType)}>{activityTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select><small>{activityTypes.find((item) => item.value === type)?.hint}</small></label><label>标题<input required value={title} placeholder={type === "training" ? "例如 周三晚训练" : "例如 周末约球"} onChange={(event) => setTitle(event.target.value)} /></label><label>开始<input required type="datetime-local" value={startsAt} onChange={(event) => { setStartsAt(event.target.value); setEndsAt(twoHoursAfter(event.target.value)); }} /></label><label>结束<input required type="datetime-local" value={endsAt} onChange={(event) => setEndsAt(event.target.value)} /></label><label>地点<input value={location} onChange={(event) => setLocation(event.target.value)} /></label>{type === "friendly" ? <label>赛制<select value={format} onChange={event=>setFormat(Number(event.target.value))}>{[5,7,8,11].map(n=><option key={n} value={n}>{n} 人制</option>)}</select></label> : null}<label>对手<input value={opponent} placeholder="友谊赛可填写" onChange={(event) => setOpponent(event.target.value)} /></label><label className="wide">活动内容（队员可见）<textarea value={notes} placeholder="填写训练项目、比赛安排、集合与装备要求，支持换行" onChange={(event) => setNotes(event.target.value)} /></label></div><button className="primary-button" disabled={busy}>{busy ? "正在建立…" : "建立活动"}<ChevronRight size={18} /></button></form>;
}

function EventEditor({ event, client, onUpdated, onError }: { event: TeamEvent; client: APIClient; onUpdated: (event: TeamEvent) => void; onError: (message: string) => void }) {
  const [open, setOpen] = useState(false); const [draft, setDraft] = useState({ type: event.type, format: event.format || 0, title: event.title, starts_at: toLocalDateTimeInput(event.starts_at), ends_at: toLocalDateTimeInput(event.ends_at), location: event.location, opponent: event.opponent, notes: event.notes }); const [busy, setBusy] = useState(false);
  async function submit(formEvent: FormEvent) { formEvent.preventDefault(); setBusy(true); try { const result = await client.updateEvent(event.id, { ...draft, starts_at: new Date(draft.starts_at).toISOString(), ends_at: new Date(draft.ends_at).toISOString() }); onUpdated(result.event); setOpen(false); } catch (err) { onError(messageFromError(err)); } finally { setBusy(false); } }
  return <details className="event-editor" open={open} onToggle={(value) => setOpen(value.currentTarget.open)}><summary>编辑活动信息</summary><form onSubmit={submit}><div className="form-grid"><label>类型<select value={draft.type} onChange={(value) => setDraft({ ...draft, type: value.target.value as EventType })}>{activityTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label><label>标题<input required value={draft.title} onChange={(value) => setDraft({ ...draft, title: value.target.value })} /></label><label>开始<input required type="datetime-local" value={draft.starts_at} onChange={(value) => setDraft({ ...draft, starts_at: value.target.value, ends_at: twoHoursAfter(value.target.value) })} /></label><label>结束<input required type="datetime-local" value={draft.ends_at} onChange={(value) => setDraft({ ...draft, ends_at: value.target.value })} /></label>{draft.type === "friendly" ? <label>赛制<select value={draft.format} onChange={e=>setDraft({...draft,format:Number(e.target.value)})}><option value={0}>未设置</option>{[5,7,8,11].map(n=><option key={n} value={n}>{n} 人制</option>)}</select></label> : null}<label>地点<input value={draft.location} onChange={(value) => setDraft({ ...draft, location: value.target.value })} /></label><label>对手<input value={draft.opponent} onChange={(value) => setDraft({ ...draft, opponent: value.target.value })} /></label><label className="wide">活动内容（队员可见）<textarea placeholder="填写训练项目、比赛安排、集合与装备要求，支持换行" value={draft.notes} onChange={(value) => setDraft({ ...draft, notes: value.target.value })} /></label></div><button className="primary-button" disabled={busy}><Save size={16} />{busy ? "保存中" : "保存修改"}</button></form></details>;
}

function PlayerActivities({client,player,events,records,onRefresh,onOpenAlbum,onError,onNavigate}: {client:APIClient;player:Player;events:TeamEvent[];records:PlayerEventAttendanceRecord[];summary:AttendanceDecisionSummary;onRefresh:()=>Promise<void>;onOpenAlbum:(target:AlbumTarget)=>void;onError:(message:string)=>void;onNavigate:(view:View)=>void}) {
  const [typeFilter,setTypeFilter]=useState<EventType>('training');const [savingID,setSavingID]=useState('');const [feedback,setFeedback]=useState<Record<string,string>>({});const [teamSummary,setTeamSummary]=useState<AttendanceDecisionSummary|null>(null);const [completedCount,setCompletedCount]=useState<number|null>(null);
  useEffect(()=>{let live=true;client.history({},true).then(result=>{if(live)setCompletedCount(result.history.filter(row=>(row.type==='training'||row.type==='friendly')&&row.rsvp_status==='available'&&!row.cancelled_at).length);}).catch(err=>onError(messageFromError(err)));return()=>{live=false;};},[client,onError]);
  const upcoming=splitEventsByTime(events).upcoming.filter(event=>!event.cancelled_at&&event.type===typeFilter);const next=upcoming[0];const currentEvent=useRef(next?.id);currentEvent.current=next?.id;
  const statusByEvent=Object.fromEntries(records.map(record=>[record.event_id,record.status]));
  useEffect(()=>{let live=true;setTeamSummary(null);if(next)client.getPlayerAttendance(next.id).then(result=>{if(live)setTeamSummary(result.summary);}).catch(err=>{if(live)onError(messageFromError(err));});return()=>{live=false;};},[client,next?.id,onError]);
  async function save(eventID:string,status:AttendanceStatus){setSavingID(eventID);setFeedback(current=>({...current,[eventID]:'正在保存…'}));try{const result=await client.savePlayerAttendance(eventID,status);if(currentEvent.current===eventID)setTeamSummary(result.summary);await onRefresh();setFeedback(current=>({...current,[eventID]:'已保存'}));}catch(err){setFeedback(current=>({...current,[eventID]:'保存失败，请重试'}));onError(messageFromError(err));}finally{setSavingID('');}}
  return <section className="page player-page"><PageHeader eyebrow="" title="活动"/><ActivityTabs value={typeFilter} onChange={setTypeFilter}/><div className="player-home-grid"><section className="overview-card player-rsvp"><EventBanner event={next}/>{next?<><div className="player-status-grid" role="group" aria-label="我的报名状态">{playerStatusOptions.map(status=><button key={status} aria-pressed={(statusByEvent[next.id]??'unknown')===status} disabled={Boolean(savingID)} className={`choice-${status} ${(statusByEvent[next.id]??'unknown')===status?'active':''}`} onClick={()=>save(next.id,status)}><span>{status==='available'?<Check size={19}/>:status==='unavailable'?<X size={19}/>:status==='tentative'?'?':'···'}</span>{attendanceLabels[status]}</button>)}</div><div className="player-save-summary"><small role="status">{feedback[next.id]??(statusByEvent[next.id]?'已保存':'点击状态即可保存')}</small><span>{teamSummary?`${teamSummary.available}人参加 · ${teamSummary.unknown}人未确认`:'正在同步报名…'}</span></div></>:null}</section><TopicPreview client={client} playerMode onOpen={()=>onNavigate('special')}/><RankingPreview client={client} playerMode onOpen={()=>onNavigate('stats')}/><AlbumPreview client={client} playerMode onOpen={album=>onOpenAlbum(album?{event_id:album.event_id,special_activity_id:album.special_activity_id}:null)}/><button className="player-milestone" onClick={()=>onNavigate('stats')}><ShieldCheck size={23}/><span>{completedCount===null?`${player.name}的训练足迹`:`已报名${completedCount}次活动`}<small>每一次参加，都值得记录</small></span><ChevronRight size={17}/></button></div><details className="event-archive"><summary>更多活动（{Math.max(0,upcoming.length-1)}）</summary><section className="player-event-cards">{upcoming.slice(1).map(event=><article className="player-event-card" key={event.id}><div><h3>{event.title}</h3><p>{formatDateRange(event)} · {event.location||'地点待定'}</p><EventContent event={event}/></div><div className="compact-rsvp">{playerStatusOptions.map(status=><button key={status} disabled={Boolean(savingID)} className={statusByEvent[event.id]===status?'active':''} onClick={()=>save(event.id,status)}>{attendanceLabels[status]}</button>)}</div><button className="icon-button" aria-label={`${event.title} 相册`} onClick={()=>onOpenAlbum({event_id:event.id})}><ImageIcon size={18}/></button><small role="status">{feedback[event.id]}</small></article>)}</section></details></section>;
}

function PlayersPanel({ client, players, locations, onPlayersChanged, onLocationsChanged, onError }: { client: APIClient; players: Player[]; locations: EventLocation[]; onPlayersChanged: (players: Player[]) => void; onLocationsChanged: (locations: EventLocation[]) => void; onError: (message: string) => void }) {
  const [query, setQuery] = useState(""); const [editing, setEditing] = useState<Player | null>(null);
  const visible = players.filter((player) => `${player.name} ${player.number ?? ""} ${player.positions.join(" ")}`.toLowerCase().includes(query.toLowerCase()));
  async function save(player: Player) { try { const result = await client.updatePlayer(player.id, { name: player.name, number: player.number, positions: player.positions, status: player.status }); onPlayersChanged(players.map((item) => item.id === player.id ? result.player : item)); setEditing(null); } catch (err) { onError(messageFromError(err)); } }
  async function remove(id: string) { if (!window.confirm("删除这个队员？有历史记录的队员应改为停用。")) return; try { await client.deletePlayer(id); onPlayersChanged(players.filter((player) => player.id !== id)); } catch (err) { onError(messageFromError(err)); } }
  async function removeLocation(id: string) { try { await client.deleteLocation(id); onLocationsChanged(locations.filter((location) => location.id !== id)); } catch (err) { onError(messageFromError(err)); } }
  return <section className="page"><PageHeader eyebrow="ROSTER" title="队员与地点" /><div className="management-layout"><div><section className="card-form"><h3>微信成员</h3><p>成员在小程序自由注册后自动加入名单。教练权限由负责人在小程序指定。</p></section><section className="location-card"><h3>常用地点</h3>{locations.map((location) => <div key={location.id}><span>{location.name}</span><button className="icon-button" onClick={() => removeLocation(location.id)} aria-label={`删除 ${location.name}`}><Trash2 size={15} /></button></div>)}{locations.length === 0 ? <small>在新建活动中添加常用地点。</small> : null}</section></div><section className="roster-card"><div className="section-heading"><div><span className="eyebrow">{players.length} PLAYERS</span><h3>球队名单</h3></div><input className="filter-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索队员" /></div>{visible.map((player) => editing?.id === player.id ? <PlayerEditor key={player.id} player={editing} onChange={setEditing} onSave={save} onCancel={() => setEditing(null)} /> : <article className="player-card" key={player.id}><strong>{player.number ?? "--"}</strong><div><h3>{player.name} {player.status === "inactive" ? <em>已停用</em> : null}</h3><p>{player.positions.join(" / ") || "未设置位置"}</p></div><div><button className="icon-button" onClick={() => setEditing({ ...player, positions: [...player.positions] })} aria-label={`编辑 ${player.name}`}><Pencil size={16} /></button><button className="icon-button" onClick={() => remove(player.id)} aria-label={`删除 ${player.name}`}><Trash2 size={16} /></button></div></article>)}{visible.length === 0 ? <Empty title="还没有队员" copy="队友使用微信登录小程序后会出现在这里。" /> : null}</section></div></section>;
}

function PlayerEditor({ player, onChange, onSave, onCancel }: { player: Player; onChange: (player: Player) => void; onSave: (player: Player) => void; onCancel: () => void }) { return <article className="player-editor"><input value={player.name} onChange={(event) => onChange({ ...player, name: event.target.value })} /><input type="number" value={player.number ?? ""} onChange={(event) => onChange({ ...player, number: event.target.value ? Number(event.target.value) : null })} /><input value={player.positions.join("，")} onChange={(event) => onChange({ ...player, positions: event.target.value.split(/[,，]/).map((item) => item.trim()).filter(Boolean) })} /><select value={player.status} onChange={(event) => onChange({ ...player, status: event.target.value as Player["status"] })}><option value="active">在队</option><option value="inactive">停用</option></select><button className="primary-button" onClick={() => onSave(player)}>保存</button><button className="secondary-button" onClick={onCancel}>取消</button></article>; }

function StatsPanel({ client, coach, player, players, onError }: { client: APIClient; coach: boolean; player: Player | null; players: Player[]; onError: (message: string) => void }) {
  const [terms, setTerms] = useState<Term[]>([]); const [scope, setScope] = useState<"month" | "term" | "year" | "half1" | "half2">("month"); const [month, setMonth] = useState(currentMonth()); const [termID, setTermID] = useState(""); const [leaderboard, setLeaderboard] = useState<Leaderboard | null>(null); const [history, setHistory] = useState<HistoryRow[]>([]); const [historyType, setHistoryType] = useState<EventType | "">(""); const [historyPlayer, setHistoryPlayer] = useState(""); const [termDraft, setTermDraft] = useState({ name: "", starts_on: "", ends_on: "" });
  const loadTerms = useCallback(async () => { try { const result = await client.listTerms(!coach); setTerms(result.terms); setTermID((current) => current || result.terms[0]?.id || ""); } catch (err) { onError(messageFromError(err)); } }, [client, coach, onError]);
  const rankingRequest=useRef(0);
  const loadLeaderboard = useCallback(async () => { const request=++rankingRequest.current;setLeaderboard(null);try { const query = scope === "month" ? { month } : scope === "term" ? { term_id: termID } : {year:month.slice(0,4),half:scope==="half1"?"1":scope==="half2"?"2":undefined}; if (scope === "term" && !termID) { setLeaderboard(null); return; } const result = await client.leaderboard(query, !coach); if(request===rankingRequest.current)setLeaderboard(result.leaderboard); } catch (err) { if(request===rankingRequest.current){setLeaderboard(null); onError(messageFromError(err));} } }, [client, coach, month, onError, scope, termID]);
  const loadHistory = useCallback(async () => { try { const query = coach ? { type: historyType || undefined, player_id: historyPlayer || undefined } : {}; const result = await client.history(query, !coach); setHistory(result.history); } catch (err) { onError(messageFromError(err)); } }, [client, coach, historyPlayer, historyType, onError]);
  useEffect(() => { void loadTerms(); }, [loadTerms]); useEffect(() => { void loadLeaderboard(); }, [loadLeaderboard]); useEffect(() => { void loadHistory(); }, [loadHistory]);
  async function createTerm(event: FormEvent) { event.preventDefault(); try { await client.createTerm(termDraft); setTermDraft({ name: "", starts_on: "", ends_on: "" }); await loadTerms(); } catch (err) { onError(messageFromError(err)); } }
  async function deleteTerm(id: string) { if (!window.confirm("删除这个学期？统计不会删除。")) return; try { await client.deleteTerm(id); await loadTerms(); } catch (err) { onError(messageFromError(err)); } }
  async function exportHistory() { try { const blob = await client.downloadHistory({ type: historyType || undefined, player_id: historyPlayer || undefined }); const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = "anypitch-history.csv"; anchor.click(); URL.revokeObjectURL(url); } catch (err) { onError(messageFromError(err)); } }
  const mine = leaderboard?.entries.find((entry) => entry.player_id === player?.id);
  return <section className="page"><PageHeader eyebrow="TRAINING RECORD" title={coach ? "数据统计" : "我的训练记录"} /><div className="stats-layout"><div><section className="leaderboard-card"><div className="section-heading"><div><span className="eyebrow">TRAINING LEADERBOARD</span><h2>训练榜</h2><p>训练与友谊赛的参加次数合计，取消活动不计。</p></div>{player ? <div className="milestone"><b>{mine?.count ?? 0}</b><span>次参加<br />下一里程碑 {nextMilestone(mine?.count ?? 0)} 次</span></div> : null}</div><div className="scope-controls"><button className={scope === "month" ? "active" : ""} onClick={() => setScope("month")}>月榜</button><button className={scope === "term" ? "active" : ""} onClick={() => setScope("term")}>学期榜</button>{["year","half1","half2"].map((v,i)=><button key={v} className={scope===v?"active":""} onClick={()=>setScope(v as typeof scope)}>{["年度","上半年","下半年"][i]}</button>)}{scope === "month" ? <input type="month" value={month} onChange={(event) => setMonth(event.target.value)} /> : scope!=="term" ? <input aria-label="榜单年份" type="number" min="1" max="9999" value={month.slice(0,4)} onChange={e=>setMonth(e.target.value+month.slice(4))}/> : <select value={termID} onChange={(event) => setTermID(event.target.value)}><option value="">选择学期</option>{terms.map((term) => <option key={term.id} value={term.id}>{term.name}</option>)}</select>}</div>{scope === "term" && terms.length === 0 ? <Empty title="还没有设置学期" copy="设置起止日期后，学期榜会按活动时间自动计算。" /> : <LeaderboardList leaderboard={leaderboard} />}</section>{coach ? <section className="term-card"><h3>学期管理</h3><form onSubmit={createTerm} className="term-form"><input required value={termDraft.name} placeholder="例如 2026年秋季学期" onChange={(event) => setTermDraft({ ...termDraft, name: event.target.value })} /><input required type="date" value={termDraft.starts_on} onChange={(event) => setTermDraft({ ...termDraft, starts_on: event.target.value })} /><input required type="date" value={termDraft.ends_on} onChange={(event) => setTermDraft({ ...termDraft, ends_on: event.target.value })} /><button className="primary-button">添加学期</button></form>{terms.map((term) => <div className="term-row" key={term.id}><span><b>{term.name}</b><small>{term.starts_on} 至 {term.ends_on}</small></span><button className="icon-button" onClick={() => deleteTerm(term.id)} aria-label={`删除 ${term.name}`}><Trash2 size={16} /></button></div>)}</section> : null}</div>
      <section className="history-card"><div className="section-heading"><div><span className="eyebrow">{coach ? "TEAM HISTORY" : "MY HISTORY"}</span><h2>{coach ? "报名历史" : "逐场记录"}</h2></div>{coach ? <button className="secondary-button" onClick={exportHistory}>导出 CSV</button> : null}</div>{coach ? <div className="history-filter"><select value={historyType} onChange={(event) => setHistoryType(event.target.value as EventType | "")}><option value="">所有类型</option>{activityTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select><select value={historyPlayer} onChange={(event) => setHistoryPlayer(event.target.value)}><option value="">所有队员</option>{players.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></div> : null}<div className="history-table">{history.map((row) => <article key={`${row.event_id}:${row.player_id}`}><div><strong>{row.title}</strong><small>{formatDate(row.starts_at)} · {eventTypeLabel(row.type)}{row.cancelled_at ? " · 已取消" : ""}</small></div>{coach ? <span>{row.player_name}</span> : null}<span className={`status-chip ${calendarEventStatusClass(row.rsvp_status)}`}>{attendanceLabels[row.rsvp_status]}</span></article>)}{history.length === 0 ? <Empty title="还没有记录" copy="报名后会在这里留下记录。" /> : null}</div></section></div></section>;
}

function LeaderboardList({ leaderboard }: { leaderboard: Leaderboard | null }) { if (!leaderboard) return <div className="loading-copy">正在计算训练榜…</div>; return <div className="leaderboard-list">{leaderboard.entries.map((entry) => <div key={entry.player_id}><b className={entry.rank <= 3 ? `rank top-${entry.rank}` : "rank"}>{entry.rank}</b><span>{entry.name}{entry.status === "inactive" ? <small>已离队</small> : null}</span><strong>{entry.count}<small>次</small></strong></div>)}{leaderboard.entries.length === 0 ? <Empty title="还没有微信成员" copy="成员注册并报名参加训练或友谊赛后，榜单会自动更新。" /> : null}</div>; }

function SpecialActivitiesPanel({ client, coach, onOpenAlbum, onError }: { client: APIClient; coach: boolean; onOpenAlbum: (target: AlbumTarget) => void; onError: (message: string) => void }) {
  const [editingTopic,setEditingTopic] = useState(false); const [entryOpen,setEntryOpen] = useState(false); const [topicSaved,setTopicSaved] = useState("");
  const [activities, setActivities] = useState<SpecialActivity[]>([]); const [selected, setSelected] = useState<SpecialActivity | null>(null); const [entries, setEntries] = useState<SpecialActivityEntry[]>([]); const [createOpen, setCreateOpen] = useState(false); const [draft, setDraft] = useState({ name: "", starts_on: "", ends_on: "", summary: "", status: "active" as "active" | "archived" }); const [entryDraft, setEntryDraft] = useState({ id: "", title: "", occurs_at: defaultStart(), location: "", opponent: "", home_score: "", away_score: "", notes: "" });
  const loadActivities = useCallback(async () => { try { const result = await client.listSpecialActivities(!coach); setActivities(result.activities); setSelected((current) => current ? result.activities.find((activity) => activity.id === current.id) ?? null : result.activities[0] ?? null); } catch (err) { onError(messageFromError(err)); } }, [client, coach, onError]);
  const loadEntries = useCallback(async () => { if (!selected) { setEntries([]); return; } try { const result = await client.listSpecialEntries(selected.id, !coach); setEntries(result.entries); } catch (err) { onError(messageFromError(err)); } }, [client, coach, onError, selected]);
  useEffect(() => { void loadActivities(); }, [loadActivities]); useEffect(() => { void loadEntries(); }, [loadEntries]);
  async function create(event: FormEvent) { event.preventDefault(); try { await client.createSpecialActivity(draft); setCreateOpen(false); setDraft({ name: "", starts_on: "", ends_on: "", summary: "", status: "active" }); await loadActivities(); } catch (err) { onError(messageFromError(err)); } }
  async function saveSelected() { if (!selected) return; try { const result = await client.updateSpecialActivity(selected.id, selected); setSelected(result.activity); setEditingTopic(false); setTopicSaved("已保存"); await loadActivities(); } catch (err) { onError(messageFromError(err)); } }
  async function archive() { if (!selected) return; try { const result = await client.updateSpecialActivity(selected.id, { status: selected.status === "active" ? "archived" : "active" }); setSelected(result.activity); await loadActivities(); } catch (err) { onError(messageFromError(err)); } }
  async function addEntry(event: FormEvent) { event.preventDefault(); if (!selected) return; const home = entryDraft.home_score === "" ? null : Number(entryDraft.home_score); const away = entryDraft.away_score === "" ? null : Number(entryDraft.away_score); if ((home === null) !== (away === null)) { onError("比分请同时填写主队和客队，或都留空。"); return; } const input = { title: entryDraft.title, occurs_at: new Date(entryDraft.occurs_at).toISOString(), location: entryDraft.location, opponent: entryDraft.opponent, home_score: home, away_score: away, notes: entryDraft.notes }; try { if (entryDraft.id) await client.updateSpecialEntry(selected.id, entryDraft.id, input); else await client.createSpecialEntry(selected.id, input); setEntryDraft({ id: "", title: "", occurs_at: defaultStart(), location: "", opponent: "", home_score: "", away_score: "", notes: "" }); setEntryOpen(false); await loadEntries(); } catch (err) { onError(messageFromError(err)); } }
  async function removeEntry(id: string) { if (!selected || !window.confirm("删除这条安排？")) return; try { await client.deleteSpecialEntry(selected.id, id); await loadEntries(); } catch (err) { onError(messageFromError(err)); } }
  return <section className="page topic-page"><PageHeader eyebrow="" title="赛事专题" action={coach ? <button className="primary-button" onClick={() => setCreateOpen((value) => !value)}><Plus size={17} />新建专题</button> : undefined} /><p className="page-intro">记录校长杯等球队活动的赛程、赛果与照片。</p>{createOpen ? <form className="event-form" onSubmit={create}><div className="form-grid"><label>专题名称<input required value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label><label>状态<select value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as "active" | "archived" })}><option value="active">进行中</option><option value="archived">已归档</option></select></label><label>开始日期<input required type="date" value={draft.starts_on} onChange={(event) => setDraft({ ...draft, starts_on: event.target.value })} /></label><label>结束日期<input required type="date" value={draft.ends_on} onChange={(event) => setDraft({ ...draft, ends_on: event.target.value })} /></label><label className="wide">简介<textarea value={draft.summary} onChange={(event) => setDraft({ ...draft, summary: event.target.value })} /></label></div><button className="primary-button">建立专题</button></form> : null}<div className="special-layout"><div className="special-list">{activities.map((activity) => <button key={activity.id} className={selected?.id === activity.id ? "active" : ""} onClick={() => { setSelected(activity); setEditingTopic(false); setEntryOpen(false); setTopicSaved(""); }}><span className={activity.status === "active" ? "live-dot" : "archive-dot"} /><div><strong>{activity.name}</strong><small>{activity.starts_on} 至 {activity.ends_on}</small></div><ChevronRight size={17} /></button>)}{activities.length === 0 ? <Empty title="还没有校长杯专题" copy="创建一届专题后，再逐条记录安排、结果和照片。" /> : null}</div><section className="special-detail">{selected ? <><div className="detail-heading"><div><span className={selected.status === "active" ? "type-badge pickup" : "type-badge"}>{selected.status === "active" ? "进行中" : "已归档"}</span>{coach && editingTopic ? <input aria-label="专题名称" className="title-input" value={selected.name} onChange={(event) => setSelected({ ...selected, name: event.target.value })} /> : <h2>{selected.name}</h2>}<p>{selected.starts_on} 至 {selected.ends_on}</p></div><button className="icon-button" onClick={() => onOpenAlbum({ special_activity_id: selected.id })} aria-label="打开专题相册"><ImageIcon size={19} /></button></div><p className="special-summary">{!editingTopic ? selected.summary || '在这里记录本届活动的安排和回忆。' : ''}</p>{coach && editingTopic ? <section className="topic-editor"><div className="form-grid"><label>开始日期<input type="date" value={selected.starts_on} onChange={e=>setSelected({...selected,starts_on:e.target.value})}/></label><label>结束日期<input type="date" value={selected.ends_on} onChange={e=>setSelected({...selected,ends_on:e.target.value})}/></label><label className="wide">专题简介<textarea value={selected.summary} onChange={e=>setSelected({...selected,summary:e.target.value})}/></label></div><div className="detail-actions"><button className="secondary-button" onClick={archive}>{selected.status==='active'?'归档本届':'恢复进行中'}</button><button className="primary-button" onClick={saveSelected}><Save size={16}/>保存专题</button></div></section> : null}<div className="topic-toolbar">{coach?<button className="secondary-button" onClick={()=>{if(editingTopic){setSelected(activities.find(a=>a.id===selected.id)??selected);}setEditingTopic(!editingTopic);}}><Pencil size={15}/>{editingTopic?'取消编辑':'编辑专题'}</button>:null}<span role="status">{topicSaved}</span><button className="secondary-button" onClick={()=>onOpenAlbum({special_activity_id:selected.id})}><ImageIcon size={16}/>活动相册</button></div><div className="section-heading"><div><h3>赛程与赛果</h3><p>{entries.length} 项安排</p></div>{coach?<button className="primary-button" onClick={()=>setEntryOpen(!entryOpen)}><Plus size={16}/>{entryOpen?'收起表单':'添加安排'}</button>:null}</div><div className="entry-list">{entries.map((entry) => <article key={entry.id}><time>{formatDate(entry.occurs_at)}</time><div><strong>{entry.title}</strong><small>{entry.location || "地点待定"}{entry.opponent ? ` · 对阵 ${entry.opponent}` : ""}</small><p>{entry.notes}</p></div><b>{entry.home_score === null ? "未录入赛果" : `${entry.home_score} : ${entry.away_score}`}</b>{coach ? <div className="entry-actions"><button className="icon-button" aria-label={`编辑 ${entry.title}`} onClick={()=>{setEntryDraft({id:entry.id,title:entry.title,occurs_at:toLocalDateTimeInput(entry.occurs_at),location:entry.location,opponent:entry.opponent,home_score:entry.home_score===null?'':String(entry.home_score),away_score:entry.away_score===null?'':String(entry.away_score),notes:entry.notes});setEntryOpen(true);}}><Pencil size={15}/></button><button className="icon-button" onClick={() => removeEntry(entry.id)} aria-label={`删除 ${entry.title}`}><Trash2 size={15} /></button></div> : null}</article>)}{entries.length === 0 ? <Empty title="还没有安排" copy="添加赛程、结果或重要说明。" /> : null}</div>{coach && entryOpen ? <form className="entry-form" onSubmit={addEntry}><h3>{entryDraft.id?"编辑安排":"添加安排"}</h3><div className="form-grid"><label>标题<input required value={entryDraft.title} onChange={(event) => setEntryDraft({ ...entryDraft, title: event.target.value })} /></label><label>时间<input required type="datetime-local" value={entryDraft.occurs_at} onChange={(event) => setEntryDraft({ ...entryDraft, occurs_at: event.target.value })} /></label><label>地点<input value={entryDraft.location} onChange={(event) => setEntryDraft({ ...entryDraft, location: event.target.value })} /></label><label>对手<input value={entryDraft.opponent} onChange={(event) => setEntryDraft({ ...entryDraft, opponent: event.target.value })} /></label><label>我方比分<input min="0" type="number" value={entryDraft.home_score} onChange={(event) => setEntryDraft({ ...entryDraft, home_score: event.target.value })} /></label><label>对手比分<input min="0" type="number" value={entryDraft.away_score} onChange={(event) => setEntryDraft({ ...entryDraft, away_score: event.target.value })} /></label><label className="wide">说明<textarea value={entryDraft.notes} onChange={(event) => setEntryDraft({ ...entryDraft, notes: event.target.value })} /></label></div><button className="primary-button">{entryDraft.id?"保存安排":"建立安排"}</button></form> : null}</> : <Empty title="选择一届专题" copy="查看其赛程、赛果和照片。" />}</section></div></section>;
}

function AlbumsPanel({ client, playerMode, target, onTargetOpened, onError }: { client: APIClient; playerMode: boolean; target: AlbumTarget; onTargetOpened: () => void; onError: (message: string) => void }) {
  const [albums, setAlbums] = useState<Album[]>([]); const [selectedID, setSelectedID] = useState(""); const [photos, setPhotos] = useState<Photo[]>([]); const [thumbs, setThumbs] = useState<Record<string, string>>({}); const [selectedPhoto, setSelectedPhoto] = useState<Photo | null>(null); const [fullURL, setFullURL] = useState(""); const [uploads, setUploads] = useState<Record<string, string>>({});
  const selected = albums.find((album) => album.id === selectedID) ?? (selectedID ? undefined : albums[0]);
  const refreshAlbums = useCallback(async () => { try { const result = await client.listAlbums(playerMode); setAlbums(result.albums); setSelectedID((current) => current || result.albums[0]?.id || ""); } catch (err) { onError(messageFromError(err)); } }, [client, onError, playerMode]);
  useEffect(() => { void refreshAlbums(); }, [refreshAlbums]);
  useEffect(() => { if (!target) return; void (async () => { try { if (playerMode) { const result = await client.listAlbums(true); setAlbums(result.albums); const album = result.albums.find((item) => target.event_id ? item.event_id === target.event_id : item.special_activity_id === target.special_activity_id); setSelectedID(album?.id ?? "missing"); } else { const result = await client.ensureAlbum(target, false); await refreshAlbums(); setSelectedID(result.album.id); } onTargetOpened(); } catch (err) { onError(messageFromError(err)); } })(); }, [client, onError, onTargetOpened, playerMode, refreshAlbums, target]);
  useEffect(() => { if (!selected?.id) { setPhotos([]); return; } void (async () => { try { const result = await client.listPhotos(selected.id, playerMode); setPhotos(result.photos); } catch (err) { onError(messageFromError(err)); } })(); }, [client, onError, playerMode, selected?.id]);
  useEffect(() => { let cancelled = false; const urls: string[] = []; void (async () => { const entries = await Promise.all(photos.map(async (photo) => { try { const blob = await client.photoBlob(photo.id, true, playerMode); const url = URL.createObjectURL(blob); urls.push(url); return [photo.id, url] as const; } catch { return null; } })); if (!cancelled) setThumbs(Object.fromEntries(entries.filter((entry): entry is readonly [string, string] => Boolean(entry)))); })(); return () => { cancelled = true; urls.forEach((url) => URL.revokeObjectURL(url)); }; }, [client, photos, playerMode]);
  async function upload(files: FileList | null) { if (!files || !selected) return; const queue = Array.from(files).slice(0, 20); for (const file of queue) { const key = `${file.name}:${file.lastModified}`; if (file.size > 6 * 1024 * 1024) { setUploads((current) => ({ ...current, [key]: "超过 6MB，未上传" })); continue; } setUploads((current) => ({ ...current, [key]: "上传中" })); try { await client.uploadPhoto(selected.id, file, "", playerMode); setUploads((current) => ({ ...current, [key]: "已上传" })); } catch (err) { setUploads((current) => ({ ...current, [key]: "上传失败" })); onError(`${file.name}：${messageFromError(err)}`); } } if (selected) { const result = await client.listPhotos(selected.id, playerMode); setPhotos(result.photos); await refreshAlbums(); } }
  async function open(photo: Photo) { setSelectedPhoto(photo); setFullURL(""); try { const blob = await client.photoBlob(photo.id, false, playerMode); setFullURL(URL.createObjectURL(blob)); } catch (err) { onError(messageFromError(err)); } }
  async function saveCaption(photo: Photo, caption: string) { try { const result = await client.updatePhotoCaption(photo.id, caption, playerMode); setPhotos((current) => current.map((item) => item.id === photo.id ? result.photo : item)); setSelectedPhoto(result.photo); } catch (err) { onError(messageFromError(err)); } }
  async function remove(photo: Photo) { if (!window.confirm("删除这张照片？")) return; try { await client.deletePhoto(photo.id, playerMode); setPhotos((current) => current.filter((item) => item.id !== photo.id)); setSelectedPhoto(null); } catch (err) { onError(messageFromError(err)); } }
  return <section className="page"><PageHeader eyebrow="TEAM MOMENTS" title="球队相册" /><p className="page-intro">{playerMode ? "队员可查看管理端发布的球队照片。" : "在管理端上传活动与专题照片。单张 JPEG、PNG、WebP 不超过 6MB。"}</p><div className="albums-layout"><aside className="album-list">{albums.map((album) => <button key={album.id} className={selected?.id === album.id ? "active" : ""} onClick={() => setSelectedID(album.id)}><span>{album.source_kind === "event" ? <CalendarDays size={16} /> : <Crown size={16} />}</span><div><strong>{album.source_title}</strong><small>{album.photo_count} 张照片</small></div></button>)}{albums.length === 0 ? <Empty title="相册会随首次上传建立" copy={playerMode ? "管理端发布照片后可在这里查看。" : "从活动详情或校长杯专题进入相册并上传。"} /> : null}</aside><section className="photo-panel">{selected ? <><div className="section-heading"><div><span className="eyebrow">{selected.source_kind === "event" ? "ACTIVITY ALBUM" : "SPECIAL ALBUM"}</span><h2>{selected.source_title}</h2></div>{!playerMode ? <label className="upload-button"><ImageIcon size={17} />选择照片<input type="file" accept="image/jpeg,image/png,image/webp" multiple onChange={(event) => { void upload(event.target.files); event.currentTarget.value = ""; }} /></label> : null}</div>{Object.keys(uploads).length ? <div className="upload-status">{Object.entries(uploads).map(([file, state]) => <span key={file}>{file} · {state}</span>)}</div> : null}<div className="photo-grid">{photos.map((photo) => <button key={photo.id} className="photo-thumb" onClick={() => open(photo)}>{thumbs[photo.id] ? <img src={thumbs[photo.id]} alt={photo.caption || `${photo.uploader_name} 上传的照片`} /> : <span>加载中</span>}</button>)}{photos.length === 0 ? <Empty title="这里还没有照片" copy={playerMode ? "这场活动暂时没有照片。" : "选择照片后会压缩保存。"} /> : null}</div></> : <Empty title="还没有可查看的相册" copy={playerMode ? "这场活动还没有管理端发布的照片。" : "请从活动或专题进入并建立相册。"} />}</section></div>{selectedPhoto ? <PhotoViewer key={selectedPhoto.id} photo={selectedPhoto} src={fullURL || thumbs[selectedPhoto.id]} canManage={!playerMode} onSaveCaption={saveCaption} onClose={() => { if (fullURL) URL.revokeObjectURL(fullURL); setFullURL(""); setSelectedPhoto(null); }} onDelete={() => remove(selectedPhoto)} /> : null}</section>;
}

function PhotoViewer({ photo, src, canManage, onSaveCaption, onClose, onDelete }: { photo: Photo; src?: string; canManage: boolean; onSaveCaption: (photo: Photo, caption: string) => Promise<void>; onClose: () => void; onDelete: () => void }) { const [caption, setCaption] = useState(photo.caption); return <div className="lightbox" role="dialog" aria-modal="true"><button className="lightbox-close" onClick={onClose} aria-label="关闭"><X /></button>{src ? <img src={src} alt={photo.caption || "球队照片"} /> : <div>正在加载原图…</div>}<div className="lightbox-meta"><span>{photo.uploader_name} · {formatDate(photo.created_at)}</span>{canManage ? <div className="caption-editor"><input aria-label="照片说明" value={caption} placeholder="添加照片说明" onChange={(event) => setCaption(event.target.value)} /><button className="secondary-button" onClick={() => void onSaveCaption(photo, caption)}>保存说明</button></div> : photo.caption ? <p>{photo.caption}</p> : null}{canManage ? <button className="text-danger" onClick={onDelete}><Trash2 size={16} />删除照片</button> : null}</div></div>; }

function TacticsPanel({ client, players, templates, boards, onBoardsChanged, onError }: { client: APIClient; players: Player[]; templates: TacticTemplate[]; boards: TacticBoard[]; onBoardsChanged: (boards: TacticBoard[]) => void; onError: (message: string) => void }) {
  const [format, setFormat] = useState<TacticFormat>(5); const formatTemplates = useMemo(() => templatesForFormat(templates, format), [templates, format]); const [templateID, setTemplateID] = useState(""); const [opponentEnabled, setOpponentEnabled] = useState(false); const [opponentTemplateID, setOpponentTemplateID] = useState(""); const activeTemplate = formatTemplates.find((template) => template.id === templateID) ?? formatTemplates[0]; const opponentTemplate = formatTemplates.find((template) => template.id === opponentTemplateID) ?? activeTemplate; const [boardName, setBoardName] = useState("我的战术板"); const [slots, setSlots] = useState<TacticSlot[]>([]); const [selectedSlotID, setSelectedSlotID] = useState(""); const [activeDragID, setActiveDragID] = useState(""); const fieldRef = useRef<HTMLDivElement | null>(null); const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }), useSensor(KeyboardSensor)); const selectedSlot = slots.find((slot) => slot.slot_id === selectedSlotID);
  useEffect(() => { if (!formatTemplates.some((template) => template.id === templateID)) setTemplateID(formatTemplates[0]?.id ?? ""); if (!formatTemplates.some((template) => template.id === opponentTemplateID)) setOpponentTemplateID(formatTemplates[0]?.id ?? ""); }, [formatTemplates, opponentTemplateID, templateID]);
  useEffect(() => { if (!activeTemplate) return; const home = homeSlotsFromTemplate(activeTemplate); setBoardName(activeTemplate.name); setSlots(opponentEnabled && opponentTemplate ? [...home, ...opponentSlotsFromTemplate(opponentTemplate)] : home); setSelectedSlotID(home[0]?.slot_id ?? ""); }, [activeTemplate?.id, opponentEnabled, opponentTemplate?.id]);
  async function saveBoard() { if (!activeTemplate) return; try { const result = await client.createBoard({ name: boardName, template_id: activeTemplate.id, opponent_template_id: opponentEnabled ? opponentTemplate?.id : "", format: activeTemplate.format, formation: activeTemplate.formation, opponent_formation: opponentEnabled ? opponentTemplate?.formation : "", slots }); onBoardsChanged([result.board, ...boards]); } catch (err) { onError(messageFromError(err)); } }
  function dragEnd(event: DragEndEvent) { const field = fieldRef.current; if (!field) { setActiveDragID(""); return; } const rect = field.getBoundingClientRect(); setSlots((current) => current.map((slot) => slot.slot_id === event.active.id ? { ...slot, ...positionAfterDragDelta(slot, event.delta, { width: rect.width, height: rect.height }) } : slot)); setActiveDragID(""); }
  if (!activeTemplate) return <section className="page"><Empty title="正在加载战术模板" copy="请稍候，或检查服务连接。" /></section>;
  const geometry = pitchGeometry(format);
  return <section className="page"><PageHeader eyebrow="" title="战术板" /><div className="tactics-layout"><section className="tactic-controls"><label>赛制<select value={format} onChange={(event) => setFormat(Number(event.target.value) as TacticFormat)}>{[5, 8, 11].map((value) => <option key={value} value={value}>{value} 人制</option>)}</select></label><label>我方模板<select value={activeTemplate.id} onChange={(event) => setTemplateID(event.target.value)}>{formatTemplates.map((template) => <option key={template.id} value={template.id}>{template.name} · {template.formation}</option>)}</select></label><label>战术名称<input value={boardName} onChange={(event) => setBoardName(event.target.value)} /></label><label>分配队员 · {selectedSlot?.label ?? "先选择位置"}<select value={selectedSlot?.side === "opponent" ? "" : selectedSlot?.player_id ?? ""} disabled={selectedSlot?.side === "opponent"} onChange={(event) => setSlots((current) => current.map((slot) => slot.slot_id === selectedSlotID && slot.side !== "opponent" ? { ...slot, player_id: event.target.value } : slot))}><option value="">未分配</option>{players.map((player) => <option key={player.id} value={player.id}>{player.number ? `${player.number} · ` : ""}{player.name}</option>)}</select></label><label className="checkbox-row"><input type="checkbox" checked={opponentEnabled} onChange={(event) => setOpponentEnabled(event.target.checked)} />显示对手站位</label>{opponentEnabled ? <label>对手模板<select value={opponentTemplate?.id ?? ""} onChange={(event) => setOpponentTemplateID(event.target.value)}>{formatTemplates.map((template) => <option key={template.id} value={template.id}>{template.name}</option>)}</select></label> : null}<button className="primary-button" onClick={saveBoard}><Save size={17} />保存战术板</button><div className="saved-boards"><h3>已保存的战术</h3>{boards.slice(0, 4).map((board) => <div key={board.id}><strong>{board.name}</strong><small>{board.format} 人制 · {board.formation}</small></div>)}{boards.length === 0 ? <small>还没有保存的战术板。</small> : null}</div></section><DndContext sensors={sensors} autoScroll={false} onDragStart={(event: DragStartEvent) => { setActiveDragID(String(event.active.id)); setSelectedSlotID(String(event.active.id)); }} onDragEnd={dragEnd} onDragCancel={() => setActiveDragID("")}><section className="pitch-shell"><div className={`pitch ${opponentEnabled ? "has-opponent" : ""}`} ref={fieldRef} style={{ "--pitch-ratio": geometry.ratio, "--pitch-max-width": geometry.maxWidth, "--pitch-aspect": Number(geometry.ratio.split(" / ")[0]) / Number(geometry.ratio.split(" / ")[1]) } as CSSProperties}><PitchMarkings />{slots.map((slot) => <DraggableSlot key={slot.slot_id} slot={slot} selected={slot.slot_id === selectedSlotID} dragging={slot.slot_id === activeDragID} player={slot.side === "opponent" ? undefined : players.find((player) => player.id === slot.player_id)} onSelect={() => setSelectedSlotID(slot.slot_id)} />)}</div></section></DndContext></div></section>;
}

function DraggableSlot({ slot, selected, dragging, player, onSelect }: { slot: TacticSlot; selected: boolean; dragging: boolean; player?: Player; onSelect: () => void }) { const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: slot.slot_id }); const move = transform ? ` translate3d(${Math.round(transform.x)}px, ${Math.round(transform.y)}px, 0)` : ""; return <button ref={setNodeRef} className={`slot-marker ${slot.side === "opponent" ? "opponent" : ""} ${selected ? "selected" : ""} ${dragging ? "dragging" : ""}`} style={{ left: `${slot.x}%`, top: `${slot.y}%`, transform: `translate(-50%, -50%)${move}` }} onClick={onSelect} {...listeners} {...attributes}><span>{player?.number ?? slot.label}</span>{player ? <small>{player.name}</small> : null}</button>; }

function PageHeader({ eyebrow, title, action }: { eyebrow: string; title: string; action?: ReactNode }) { return <header className="page-header"><div><span className="eyebrow">{eyebrow}</span><h1>{title}</h1></div>{action}</header>; }
function NavButton({ active, icon, label, onClick }: { active: boolean; icon: ReactNode; label: string; onClick: () => void }) { return <button className={`nav-button ${active ? "active" : ""}`} onClick={onClick}>{icon}<span>{label}</span></button>; }
function Empty({ title, copy }: { title: string; copy: string }) { return <div className="empty-state"><strong>{title}</strong><p>{copy}</p></div>; }
function messageFromError(error: unknown) { return error instanceof Error ? error.message : "请求失败，请稍后再试。"; }
function eventTypeLabel(type: EventType) { return activityTypes.find((item) => item.value === type)?.label ?? type; }
function formatDate(value: string) { return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(new Date(value)); }
function formatDateRange(event: Pick<TeamEvent, "starts_at" | "ends_at">) { const start = new Date(event.starts_at); const end = new Date(event.ends_at); const date = new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit" }).format(start); const clock = new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false }); return `${date} ${clock.format(start)}–${clock.format(end)}`; }
function sortEvents(left: TeamEvent, right: TeamEvent) { return new Date(left.starts_at).getTime() - new Date(right.starts_at).getTime(); }
function nextEvent(events: TeamEvent[]) { return events.filter((event) => !event.cancelled_at && new Date(event.ends_at) >= new Date()).sort(sortEvents)[0]; }
function upsertRecord(records: AttendanceRecord[], record: AttendanceRecord) { const existing = records.some((item) => item.player_id === record.player_id); return existing ? records.map((item) => item.player_id === record.player_id ? record : item) : [...records, record]; }
function currentMonth() { const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit" }).formatToParts(new Date()); const value = (kind: "year" | "month") => parts.find((part) => part.type === kind)?.value ?? ""; return `${value("year")}-${value("month")}`; }
function nextMilestone(count: number) { return [5, 10, 20, 30, 50, 100].find((item) => item > count) ?? count + 10; }
function defaultStart() { const date = new Date(); date.setMinutes(0, 0, 0); date.setHours(date.getHours() + 1); return toLocalDateTimeInput(date); }
