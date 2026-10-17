import { PitchMarkings } from "./PitchMarkings";
import { useEffect, useState } from "react";
import { CalendarDays, ChevronLeft, ChevronRight, Trophy, Image, MapPin } from "lucide-react";
import { APIClient, type Album, type EventType, type Leaderboard, type SpecialActivity, type TacticBoard, type TeamEvent, type Term } from "./api";

export function TeamCrest() {
  return <img className="team-crest" src="/team-logo.jpg" alt="西土城FC 队徽" />;
}

export function ActivityTabs({ value, onChange }: { value: EventType; onChange: (type: EventType) => void }) {
  return <div className="activity-tabs" role="tablist" aria-label="活动类型">{([['training','训练'],['friendly','友谊赛'],['pickup','野球']] as const).map(([type,label]) => <button role="tab" aria-selected={type === value} className={type === value ? 'active' : ''} key={type} onClick={() => onChange(type)}>{label}</button>)}</div>;
}

export function EventBanner({ event }: { event?: TeamEvent }) {
  return <section className="event-banner"><h3>{event ? `下次${({training:'训练',friendly:'友谊赛',pickup:'野球'})[event.type]}` : '下一场活动'}</h3><h2>{event?.title ?? '等待新的安排'}</h2>{event ? <><p><CalendarDays size={16}/>{new Date(event.starts_at).toLocaleDateString('zh-CN',{month:'2-digit',day:'2-digit',weekday:'short'})}　{new Date(event.starts_at).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'})}–{new Date(event.ends_at).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'})}</p><p><MapPin size={16}/>{event.location || '地点待定'}</p><EventContent event={event}/></> : <p>发布活动后，队员就可以确认参与状态。</p>}</section>;
}

export function EventContent({ event }: { event: TeamEvent }) {
  const title = event.type === 'training' ? '训练内容' : event.type === 'friendly' ? '比赛安排' : '活动说明';
  return <section className="event-content" aria-label={title}>
    <h4>{title}</h4>
    <div className="event-content-scroll" role="region" aria-label={`${title}详情，可上下滚动`} tabIndex={0}>
      {event.opponent.trim() ? <p className="event-opponent">对手：{event.opponent}</p> : null}
      <p>{event.notes.trim() || '教练尚未填写具体内容'}</p>
    </div>
  </section>;
}

export function WeekSchedule({ events, selected, onSelect }: { events: TeamEvent[]; selected?: TeamEvent; onSelect: (id: string) => void }) {
  const [anchor, setAnchor] = useState(() => new Date());
  useEffect(() => { if(selected) setAnchor(new Date(selected.starts_at)); }, [selected?.id]);
  const monday = new Date(anchor); monday.setDate(monday.getDate() - (monday.getDay() + 6) % 7); monday.setHours(0,0,0,0);
  const move = (delta:number) => {const date=new Date(anchor); date.setDate(date.getDate()+delta); setAnchor(date);};
  return <section className="overview-card week-card"><div className="overview-heading"><h3>本周安排</h3><div className="week-controls"><button aria-label="上一周" onClick={()=>move(-7)}><ChevronLeft size={16}/></button><span>{monday.getMonth()+1}月</span><button aria-label="下一周" onClick={()=>move(7)}><ChevronRight size={16}/></button></div></div><div className="week-grid">{Array.from({length:7},(_,index)=>{const day=new Date(monday);day.setDate(day.getDate()+index); const list=events.filter(e=>new Date(e.starts_at).toDateString()===day.toDateString());return <div key={index} className={`week-day ${list.some(e=>e.id===selected?.id)?'selected':''}`}><time>{day.getMonth()+1}月{day.getDate()}日</time><span>{['周一','周二','周三','周四','周五','周六','周日'][index]}</span>{list.length?list.map(e=><button className={e.cancelled_at?'cancelled':''} key={e.id} onClick={()=>onSelect(e.id)} title={e.title}><b>{e.cancelled_at?'已取消':({training:'训练',friendly:'友谊赛',pickup:'野球'})[e.type]}</b><small>{new Date(e.starts_at).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'})}</small></button>):<span className="rest-day">—</span>}</div>;})}</div></section>;
}

export function TacticPreview({ board, onOpen }: { board?: TacticBoard; onOpen: () => void }) {
  const labels: Record<string,string> = {'中锋':'ST','左边锋':'LW','右边锋':'RW','左中场':'CM','右中场':'CM','后腰':'DM','左后卫':'LB','右后卫':'RB','左中卫':'CB','右中卫':'CB','门将':'GK'};
  return <section className="overview-card tactic-preview"><div className="overview-heading"><h3>战术板</h3><button className="link-button" onClick={onOpen}>{board?'编辑':'建立战术'}</button></div><button className="mini-pitch" onClick={onOpen} aria-label="打开战术板"><PitchMarkings />{board?.slots.map(slot=><span key={slot.slot_id} className={`mini-marker ${slot.side??'home'}`} style={{left:`${slot.x}%`,top:`${slot.y}%`}}>{labels[slot.label]??slot.label}</span>)}{!board?<span className="pitch-empty">安排阵型<br/><small>点击建立第一块战术板</small></span>:null}</button>{board?<p className="preview-note">{board.name} · {board.formation}</p>:null}</section>;
}

export function RankingPreview({ client, playerMode=false, onOpen }: {client:APIClient;playerMode?:boolean;onOpen:()=>void}) {
  const [scope,setScope]=useState<'month'|'term'>('month'); const [month,setMonth]=useState(()=>new Date().toLocaleDateString('sv-SE').slice(0,7)); const [terms,setTerms]=useState<Term[]>([]);const [term,setTerm]=useState('');const [ranking,setRanking]=useState<Leaderboard|null>(null);const [error,setError]=useState('');
  useEffect(()=>{let live=true;client.listTerms(playerMode).then(r=>{if(live){setTerms(r.terms);const today=new Date().toLocaleDateString('sv-SE');setTerm((r.terms.find(t=>t.starts_on<=today&&t.ends_on>=today)??r.terms[0])?.id??'');}}).catch(e=>{if(live)setError(e.message);});return()=>{live=false;};},[client,playerMode]);
  useEffect(()=>{let live=true;setRanking(null);setError('');if(scope==='term'&&!term)return;client.leaderboard(scope==='month'?{month}:{term_id:term},playerMode).then(r=>{if(live)setRanking(r.leaderboard);}).catch(e=>{if(live)setError(e.message);});return()=>{live=false;};},[client,scope,month,term,playerMode]);
  const move=(delta:number)=>{const date=new Date(`${month}-01T12:00:00`);date.setMonth(date.getMonth()+delta);setMonth(date.toLocaleDateString('sv-SE').slice(0,7));};
  return <section className="overview-card ranking-preview"><div className="overview-heading"><h3>训练榜</h3></div><div className="ranking-tabs"><button className={scope==='month'?'active':''} onClick={()=>setScope('month')}>月度</button><button className={scope==='term'?'active':''} onClick={()=>setScope('term')}>学期</button></div>{scope==='month'?<div className="ranking-period"><button aria-label="上个月" onClick={()=>move(-1)}><ChevronLeft size={16}/></button><strong>{month.replace('-','年')}月</strong><button aria-label="下个月" onClick={()=>move(1)}><ChevronRight size={16}/></button></div>:<select aria-label="榜单学期" value={term} onChange={e=>setTerm(e.target.value)}>{terms.length?terms.map(t=><option key={t.id} value={t.id}>{t.name}</option>):<option value="">尚未设置学期</option>}</select>}<div className="leaderboard-list">{ranking?.entries.slice(0,3).map(row=><div key={row.player_id}><b className={`rank top-${row.rank}`}>{row.rank}</b><span>{row.name}</span><strong>{row.count}<small>次</small></strong></div>)}</div>{error?<p role="alert" className="preview-note">{error}</p>:ranking&&!ranking.entries.length?<p className="preview-note">本期暂无训练报名</p>:!ranking&&!(scope==='term'&&!term)?<p className="preview-note">正在加载榜单…</p>:null}<footer><small>训练与友谊赛参加次数合计</small><button className="link-button" onClick={onOpen}>查看完整榜单 <ChevronRight size={14}/></button></footer></section>;
}

export function TopicPreview({client,playerMode=false,onOpen}:{client:APIClient;playerMode?:boolean;onOpen:()=>void}) {
  const [topic,setTopic]=useState<SpecialActivity>();
  useEffect(()=>{let live=true;client.listSpecialActivities(playerMode).then(r=>{if(live)setTopic(r.activities.find(a=>a.status==='active'));}).catch(()=>{});return()=>{live=false;};},[client,playerMode]);
  if (!topic) return null;
  return <button className="topic-preview overview-card" onClick={onOpen}><Trophy size={24}/><div><strong>{topic.name}</strong><small>{topic.starts_on} 至 {topic.ends_on}</small></div><span>查看赛程 <ChevronRight size={14}/></span></button>;
}

export function AlbumPreview({client,playerMode=false,onOpen}:{client:APIClient;playerMode?:boolean;onOpen:(album?:Album)=>void}) {
  const [album,setAlbum]=useState<Album>();const [images,setImages]=useState<{url:string;caption:string}[]>([]);const [error,setError]=useState('');
  useEffect(()=>{let live=true;const urls:string[]=[];async function load(){try{const result=await client.listAlbums(playerMode);const latest=result.albums.filter(a=>a.photo_count>0).sort((a,b)=>b.updated_at.localeCompare(a.updated_at))[0];if(!live||!latest)return;setAlbum(latest);const photos=await client.listPhotos(latest.id,playerMode);const thumbs=await Promise.all(photos.photos.slice(0,4).map(async p=>{const blob=await client.photoBlob(p.id,true,playerMode);const url=URL.createObjectURL(blob);if(!live)URL.revokeObjectURL(url);else urls.push(url);return {url,caption:p.caption||latest.source_title};}));if(live)setImages(thumbs);}catch(e){if(live)setError(e instanceof Error?e.message:'相册加载失败');}}void load();return()=>{live=false;urls.forEach(url=>URL.revokeObjectURL(url));};},[client,playerMode]);
  return <section className="overview-card album-preview"><div className="overview-heading"><h3>活动相册</h3><span>{album?`${album.source_title} · ${album.photo_count}张照片`:''}</span><button className="link-button mobile-upload" onClick={()=>onOpen(album)}>{playerMode ? "查看相册" : "上传照片"} <ChevronRight size={14}/></button></div><div className="album-preview-body">{images.length?<div className="album-preview-grid">{images.map(photo=><button key={photo.url} onClick={()=>onOpen(album)}><img src={photo.url} alt={photo.caption}/></button>)}</div>:<button className="album-preview-empty" onClick={()=>onOpen(album)}><Image size={25}/><span>{error||(playerMode ? '暂无管理端发布的照片' : '留下球队的第一张活动照片')}<small>训练、比赛和球场边的回忆</small></span></button>}{!playerMode ? <div className="album-upload"><button className="secondary-button" onClick={()=>onOpen(album)}>＋ 上传照片</button><small>上传后队内可见</small></div> : null}</div></section>;
}
