const api = require('../../utils/api');
const view = require('../../utils/view');
Page({
 onReady() { getApp().initTitleFont(); },
  data: { player: {}, version: '', error: '', diagnostics: false },
  async onShow() {
    if (!view.requireSession()) return;
    const miniProgram = wx.getAccountInfoSync().miniProgram;
    const version = miniProgram.version || ({ develop: '开发版', trial: '体验版' }[miniProgram.envVersion] || '未知版本');
    this.setData({ version, error: '', diagnostics: false }); this.versionTaps = 0; this.lastVersionTap = 0;
    try { this.setData({ player: await api.refreshMe() }); } catch (e) { view.fail(this, e); }
  },
  versionTap() {
    const now = Date.now();
    this.versionTaps = now - (this.lastVersionTap || 0) <= 2000 ? (this.versionTaps || 0) + 1 : 1;
    this.lastVersionTap = now;
    if (this.versionTaps < 5 || !this.data.player.id) return;
    this.versionTaps = 0;
    this.setData({ diagnostics: true });
    wx.setClipboardData({ data: this.data.player.id, success: () => view.notice('成员编号已复制'), fail: () => view.notice('复制失败，请长按编号复制') });
  },
  async logout() {
    const result = await new Promise(resolve => wx.showModal({ title: '退出登录？', success: resolve, fail: () => resolve({ confirm: false }) }));
    if (!result.confirm) return;
    try { await api.logout(); } catch (_) {}
    wx.reLaunch({ url: '/pages/login/index' });
  }
});
