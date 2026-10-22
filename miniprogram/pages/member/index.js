const api = require('../../utils/api');
const view = require('../../utils/view');

Page({
  data: { loading: true, error: '', member: null, avatarPath: '' },
  onReady() { getApp().initTitleFont(); },
  onLoad(query) { this.memberID = query.id || ''; },
  onShow() { this.active = true; if (view.requireSession('/pages/member/index?id=' + encodeURIComponent(this.memberID))) this.load(); },
  onUnload() { this.active = false; },
  async load() {
    this.setData({ loading: true, error: '' });
    try {
      const result = await api.request('/api/player/members/' + encodeURIComponent(this.memberID));
      if (!this.active) return;
      this.setData({ member: { ...result.member, initial: result.member.name.slice(0, 1) }, avatarPath: '' });
      if (result.member.has_avatar) {
        const avatarPath = await api.avatar(result.member.id);
        if (this.active) this.setData({ avatarPath });
      }
    } catch (error) { if (this.active) view.fail(this, error); }
    finally { if (this.active) { this.setData({ loading: false }); wx.stopPullDownRefresh(); } }
  },
  previewAvatar() {
    if (this.data.avatarPath) wx.previewImage({ current: this.data.avatarPath, urls: [this.data.avatarPath] });
  },
  onPullDownRefresh() { this.load(); }
});
