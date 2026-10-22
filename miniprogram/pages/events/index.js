const api = require('../../utils/api');
const view = require('../../utils/view');
const calendar = require('../../utils/calendar');

const { typeChoices: types } = require('../../utils/design');
const places = [
  { value: 'all', label: '全部地点' },
  { value: 'haidian', label: '海淀校区' },
  { value: 'shahe', label: '沙河校区' },
  { value: 'other', label: '其他' }
];
function matches(event, place, type) {
  const location = event.location || '';
  const campus = location.includes('海淀校区') ? 'haidian' : location.includes('沙河校区') ? 'shahe' : 'other';
  return (place === 'all' || campus === place) && (type === 'all' || event.type === type);
}

Page({
 onReady() { getApp().initTitleFont(); },
  data: {
    loading: true, error: '', events: [], visible: [], mode: 'current',
    place: 'all', type: 'all', placeLabel: '全部地点', typeLabel: '全部',
    places, types, filterOpen: false, dropdown: '', draftPlace: 'all', draftType: 'all', draftCount: 0,
    calendarOpen: false, calendarMonth: view.monthNow(), calendarMonthLabel: calendar.monthLabel(view.monthNow()), calendarWeeks: [], weekLabels: calendar.weekLabels, selectedDate: ''
  },
  onShow() { view.selectTab(this, 0); if (view.requireSession()) this.load(); },
  async load() {
    const seq = (this.seq || 0) + 1;
    this.seq = seq;
    this.setData({ loading: true, error: '' });
    try {
      const result = await api.request('/api/player/events');
      if (seq !== this.seq) return;
      this.previewCache = {};
      this.previewErrors = {};
      const events = view.eventRows(result.events, result.attendance_records);
      this.setData({ events });
      this.select();
      await this.loadPreviews(seq);
    } catch (e) { if (seq === this.seq) view.fail(this, e); }
    finally { if (seq === this.seq) { this.setData({ loading: false }); wx.stopPullDownRefresh(); } }
  },
  select() {
    const isHistory = this.data.mode === 'history';
    const visible = this.data.events.filter(event =>
      matches(event, this.data.place, this.data.type) &&
      (isHistory ? event.cancelled || event.ended : !event.cancelled && !event.ended)
    );
    if (isHistory) visible.reverse();
    this.setData({ calendarWeeks: calendar.monthWeeks(this.data.calendarMonth, visible, this.data.selectedDate), visible: visible.map(event => ({
      ...event,
      shortTime: view.dateText(event.starts_at),
      attendeeCount: this.previewCache?.[event.id]?.attendeeCount,
      previewFailed: Boolean(this.previewErrors?.[event.id]),
      attendeeExtra: this.previewCache?.[event.id]?.attendeeExtra,
      attendeePreview: this.previewCache?.[event.id]?.attendeePreview || []
    })) });
  },
  async loadPreviews(seq) {
    const previewSeq = (this.previewSeq || 0) + 1;
    this.previewSeq = previewSeq;
    const events = this.data.visible.filter(event => !this.previewCache?.[event.id]);
    const results = await Promise.allSettled(events.map(event => api.request('/api/player/events/' + encodeURIComponent(event.id))));
    if (seq !== this.seq || previewSeq !== this.previewSeq) return;
    this.previewCache = this.previewCache || {};
    results.forEach((result, index) => {
      if (result.status !== 'fulfilled') {
        this.previewErrors = this.previewErrors || {};
        this.previewErrors[events[index].id] = true;
        return;
      }
      if (this.previewErrors) delete this.previewErrors[events[index].id];
      const attending = (result.value.members || []).filter(member => member.status === 'available');
      this.previewCache[events[index].id] = {
        attendeeCount: attending.length,
        attendeePreview: attending.slice(0, 3).map(member => ({ id: member.player_id, initial: (member.name || '球').slice(0, 1), hasAvatar: member.has_avatar, avatarPath: '' })),
        attendeeExtra: Math.max(0, attending.length - 3)
      };
    });
    this.select();
    const avatarIDs = [...new Set(Object.values(this.previewCache).flatMap(row => row.attendeePreview.filter(member => member.hasAvatar && !member.avatarPath).map(member => member.id)))];
    await Promise.allSettled(avatarIDs.map(async id => {
      const avatarPath = await api.avatar(id);
      if (seq !== this.seq || previewSeq !== this.previewSeq) return;
      Object.values(this.previewCache).forEach(row => row.attendeePreview.forEach(member => { if (member.id === id) member.avatarPath = avatarPath; }));
      this.select();
    }));
  },
  changeMode(e) {
    this.setData({ mode: e.currentTarget.dataset.value, dropdown: '', selectedDate: '' });
    this.select();
    this.loadPreviews(this.seq);
  },
  toggleCalendar() { this.setData({ calendarOpen: !this.data.calendarOpen, dropdown: '' }); },
  moveMonth(e) {
    const month = calendar.shiftMonth(this.data.calendarMonth, Number(e.currentTarget.dataset.delta));
    this.setData({ calendarMonth: month, calendarMonthLabel: calendar.monthLabel(month), selectedDate: '' });
    this.select();
  },
  jumpToday() {
    const month = view.monthNow();
    this.setData({ calendarMonth: month, calendarMonthLabel: calendar.monthLabel(month), selectedDate: '' });
    this.select();
  },
  chooseDay(e) {
    const key = e.currentTarget.dataset.date;
    const target = this.data.visible.find(event => calendar.dateKey(event.starts_at) === key);
    this.setData({ selectedDate: key, calendarWeeks: calendar.monthWeeks(this.data.calendarMonth, this.data.visible, key) }, () => {
      if (target) wx.pageScrollTo({ selector: '#event-' + target.id, duration: 280 });
      else view.notice('这一天没有符合条件的活动');
    });
  },
  openMember(e) { wx.navigateTo({ url: '/pages/member/index?id=' + encodeURIComponent(e.currentTarget.dataset.id) }); },
  toggleDropdown(e) {
    const kind = e.currentTarget.dataset.kind;
    this.setData({ dropdown: this.data.dropdown === kind ? '' : kind });
  },
  closeDropdown() { this.setData({ dropdown: '' }); },
  chooseQuick(e) {
    const kind = e.currentTarget.dataset.kind;
    const value = e.currentTarget.dataset.value;
    const row = (kind === 'place' ? places : types).find(choice => choice.value === value);
    this.setData(kind === 'place' ? { place: value, placeLabel: row.label, dropdown: '', selectedDate: '' } : { type: value, typeLabel: row.label, dropdown: '', selectedDate: '' });
    this.select();
    this.loadPreviews(this.seq);
  },
  openFilter() {
    const tabBar = this.getTabBar && this.getTabBar();
    if (tabBar) tabBar.setData({ hidden: true });
    this.setData({ filterOpen: true, dropdown: '', draftPlace: this.data.place, draftType: this.data.type });
    this.updateDraftCount();
  },
  closeFilter() {
    this.setData({ filterOpen: false });
    const tabBar = this.getTabBar && this.getTabBar();
    if (tabBar) tabBar.setData({ hidden: false });
  },
  stop() {},
  choosePlace(e) { this.setData({ draftPlace: e.currentTarget.dataset.value }); this.updateDraftCount(); },
  chooseType(e) { this.setData({ draftType: e.currentTarget.dataset.value }); this.updateDraftCount(); },
  updateDraftCount() {
    const isHistory = this.data.mode === 'history';
    this.setData({ draftCount: this.data.events.filter(event => matches(event, this.data.draftPlace, this.data.draftType) && (isHistory ? event.cancelled || event.ended : !event.cancelled && !event.ended)).length });
  },
  resetFilter() { this.setData({ draftPlace: 'all', draftType: 'all' }); this.updateDraftCount(); },
  applyFilter() {
    this.closeFilter();
    const place = this.data.draftPlace;
    const type = this.data.draftType;
    this.setData({ place, type, placeLabel: places.find(row => row.value === place).label, typeLabel: types.find(row => row.value === type).label, filterOpen: false, selectedDate: '' });
    this.select();
    this.loadPreviews(this.seq);
  },
  topics() { wx.navigateTo({ url: '/pages/topics/index' }); },
  open(e) { wx.navigateTo({ url: '/pages/event-detail/index?id=' + encodeURIComponent(e.currentTarget.dataset.id) }); },
  onPullDownRefresh() { this.load(); },
  onUnload() { this.seq = (this.seq || 0) + 1; }
});
