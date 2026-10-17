const api = require('./api');
const { eventTypes, eventDesign } = require('./design');
const statuses = [{ value: 'available', label: '参加' }, { value: 'unavailable', label: '不参加' }, { value: 'tentative', label: '待定' }, { value: 'unknown', label: '未确认' }];
const types = Object.fromEntries(Object.entries(eventTypes).map(([key, item]) => [key, item.label]));
function pad(n) { return String(n).padStart(2, '0'); }
function chinaDate(value) { const date = new Date(value); return Number.isNaN(date.getTime()) ? null : new Date(date.getTime() + 8 * 3600000); }
function dateText(value) {
  const d = chinaDate(value);
  return d ? `${d.getUTCMonth() + 1}月${d.getUTCDate()}日 ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}` : '时间待定';
}
function monthNow() { const d = chinaDate(new Date().toISOString()); return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}`; }
function eventDateParts(value) {
  const d = chinaDate(value);
  return d ? { dateMonth: `${d.getUTCMonth() + 1}月`, dateDay: pad(d.getUTCDate()), dateClock: `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}` } : { dateMonth: '待定', dateDay: '--', dateClock: '--:--' };
}
function eventRows(events, records, now = Date.now()) {
  const byID = Object.fromEntries((records || []).map(r => [r.event_id, r.status]));
  return (events || []).map(e => ({ ...e, ...eventDateParts(e.starts_at), typeLabel: eventDesign(e.type).label, typeIcon: eventDesign(e.type).icon, typeTone: eventDesign(e.type).tone, timeText: `${dateText(e.starts_at)} — ${dateText(e.ends_at)}`, status: byID[e.id] || 'unknown', ended: new Date(e.ends_at).getTime() <= now, cancelled: Boolean(e.cancelled_at) })).sort((a, b) => new Date(a.starts_at) - new Date(b.starts_at));
}
function participationCount(history) {
  return (history || []).filter(row => row.rsvp_status === 'available' && !row.cancelled_at).length;
}
function requireSession(returnTo) {
  if (api.session()) return true;
  if (returnTo) getApp().globalData.postLoginRoute = returnTo;
  wx.reLaunch({ url: '/pages/login/index' }); return false;
}
function selectTab(page, selected) {
  const tabBar = page.getTabBar && page.getTabBar();
  if (tabBar) tabBar.setData({ selected, hidden: false });
}
function fail(page, error) {
  if (error.status === 401) { wx.reLaunch({ url: '/pages/login/index' }); return; }
  page.setData({ error: error.message || '加载失败，请重试' });
}
function notice(message) { wx.showToast({ title: message, icon: 'none', duration: 2500 }); }
module.exports = { statuses, types, dateText, eventDateParts, monthNow, eventRows, participationCount, requireSession, selectTab, fail, notice };
