const api = require('../../utils/api');
const view = require('../../utils/view');
const share = require('../../utils/share');
const labels = Object.fromEntries(view.statuses.map(item => [item.value, item.label]));

Page({
  onReady() { this.shareCanvasReady = true; getApp().initTitleFont(); if (this.data.event) this.prepareShareImage(this.data.event); },
  data: { loading: true, saving: false, error: '', event: null, shareImagePath: '', statuses: view.statuses, confirmed: [], others: [], otherGroups: [], profileComplete: false },
  onLoad(query) { this.id = query.id || ''; },
  onShow() {
    this.active = true;
    if (view.requireSession(share.eventPath(this.id))) {
      this.setData({ profileComplete: !!api.session().player.profile_complete });
      this.load();
    }
  },
  onUnload() { this.active = false; },
  async load() {
    this.setData({ loading: true, error: '', shareImagePath: '' });
    try {
      const result = await api.request('/api/player/events/' + encodeURIComponent(this.id));
      if (!this.active) return;
      const event = view.eventRows([result.event], [{ event_id: this.id, status: result.record.status }])[0];
      const members = result.members.map(member => ({ ...member, initial: (member.name || '球').slice(0, 1), avatarPath: '' }));
      const confirmed = members.filter(member => member.status === 'available');
      const others = members.filter(member => member.status !== 'available');
      const otherGroups = ['tentative', 'unavailable', 'unknown'].map(status => ({ status, label: labels[status], members: others.filter(member => member.status === status) })).filter(group => group.members.length);
      this.setData({ event, confirmed, others, otherGroups });
      if (this.shareCanvasReady) this.prepareShareImage(event);
      await Promise.allSettled(members.filter(member => member.has_avatar).map(async member => {
        const avatarPath = await api.avatar(member.player_id);
        if (!this.active) return;
        this.setData({
          confirmed: this.data.confirmed.map(row => row.player_id === member.player_id ? { ...row, avatarPath } : row),
          otherGroups: this.data.otherGroups.map(group => ({ ...group, members: group.members.map(row => row.player_id === member.player_id ? { ...row, avatarPath } : row) }))
        });
      }));
    } catch (error) { if (this.active) { if (error.status === 401) getApp().globalData.postLoginRoute = share.eventPath(this.id); view.fail(this, error); } }
    finally { if (this.active) { this.setData({ loading: false }); wx.stopPullDownRefresh(); } }
  },
  async prepareShareImage(event) {
    const seq = (this.shareImageSeq || 0) + 1;
    this.shareImageSeq = seq;
    try {
      const imagePath = await share.drawEventImage(this, event);
      if (this.active && seq === this.shareImageSeq && this.data.event && this.data.event.id === event.id) this.setData({ shareImagePath: imagePath });
    } catch (_) { if (this.active && seq === this.shareImageSeq) this.setData({ shareImagePath: '' }); }
  },
  retryShareImage() { if (this.data.event) this.prepareShareImage(this.data.event); },
  async rsvp(event) {
    if (this.data.saving || !this.data.event) return;
    if (!this.data.profileComplete) { this.completeProfile(); return; }
    this.setData({ saving: true, error: '' });
    try {
      await api.request('/api/player/events/' + encodeURIComponent(this.id) + '/attendance', 'PUT', { status: event.currentTarget.dataset.status });
      await this.load();
      view.notice('报名已保存');
    } catch (error) { view.fail(this, error); }
    finally { this.setData({ saving: false }); }
  },
  completeProfile() {
    getApp().globalData.editProfileOnShow = true;
    getApp().globalData.postProfileRoute = '/pages/event-detail/index?id=' + encodeURIComponent(this.id);
    wx.switchTab({ url: '/pages/mine/index' });
  },
  onShareAppMessage() {
    const event = this.data.event;
    return { title: event ? `${event.title}｜${view.dateText(event.starts_at)}` : '吸土城FC活动', path: share.eventPath(this.id), imageUrl: this.data.shareImagePath || share.eventBaseImage };
  },
  onPullDownRefresh() { this.load(); }
});
