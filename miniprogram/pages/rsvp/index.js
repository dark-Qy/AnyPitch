const api = require('../../utils/api');
const view = require('../../utils/view');

Page({
  onReady() { getApp().initTitleFont(); },
  data: { loading: true, error: '', events: [], statuses: view.statuses, savingID: '', profileComplete: false },
  onShow() {
    view.selectTab(this, 1);
    if (view.requireSession()) this.load();
  },
  async load() {
    const seq = (this.seq || 0) + 1;
    this.seq = seq;
    this.setData({ loading: true, error: '', profileComplete: !!api.session().player.profile_complete });
    try {
      const result = await api.request('/api/player/events');
      if (seq !== this.seq) return;
      this.setData({ events: view.eventRows(result.events, result.attendance_records).filter(event => !event.cancelled && !event.ended) });
    } catch (error) {
      if (seq === this.seq) view.fail(this, error);
    } finally {
      if (seq === this.seq) { this.setData({ loading: false }); wx.stopPullDownRefresh(); }
    }
  },
  async rsvp(event) {
    if (this.data.savingID) return;
    const id = event.currentTarget.dataset.id;
    const status = event.currentTarget.dataset.status;
    if (!this.data.profileComplete) {
      getApp().globalData.editProfileOnShow = true;
      getApp().globalData.postProfileRoute = '/pages/event-detail/index?id=' + encodeURIComponent(id);
      wx.switchTab({ url: '/pages/mine/index' });
      return;
    }
    this.setData({ savingID: id, error: '' });
    try {
      const result = await api.request('/api/player/events/' + encodeURIComponent(id) + '/attendance', 'PUT', { status });
      this.setData({ events: this.data.events.map(item => item.id === id ? { ...item, status: result.record.status } : item) });
      view.notice('报名已保存');
    } catch (error) { view.fail(this, error); }
    finally { this.setData({ savingID: '' }); }
  },
  open(event) { wx.navigateTo({ url: '/pages/event-detail/index?id=' + encodeURIComponent(event.currentTarget.dataset.id) }); },
  onPullDownRefresh() { this.load(); },
  onUnload() { this.seq = (this.seq || 0) + 1; }
});
