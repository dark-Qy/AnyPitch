const api = require('../../utils/api');
const view = require('../../utils/view');
const { eventDesign } = require('../../utils/design');
const share = require('../../utils/share');
const AVATAR_REVIEW_KEY = 'anypitch_avatar_review';
const AVATAR_MAX_BYTES = 6 * 1024 * 1024;
const NOTICE_PENDING_KEY = 'anypitch_notice_pending_grant';
function setTabBarHidden(page, hidden) { const tabBar = page.getTabBar && page.getTabBar(); if (tabBar) tabBar.setData({ hidden }); }

Page({
 onReady() { getApp().initTitleFont(); },
  data: { loading: true, saving: false, editing: false, uploadingAvatar: false, avatarStatus: '', notificationBusy: false, notificationReady: false, notificationRemaining: 0, notificationTemplateID: '', notificationHint: '暂未开放', notificationStatus: '', error: '', player: {}, initial: '我', avatarPath: '', history: [], recentHistory: [], participationCount: 0 },
  async onShow() { this.avatarReviewActive = true; view.selectTab(this, 3); if (!view.requireSession()) return; await this.load(); this.refreshNotifications(); this.resumeAvatarReview(); if (getApp().globalData.editProfileOnShow) { getApp().globalData.editProfileOnShow = false; this.edit(); } },
  async refreshNotifications() {
    const pending = wx.getStorageSync(NOTICE_PENDING_KEY);
    if (pending && pending.playerID === this.data.player.id) {
      try {
        await api.request('/api/player/notifications', 'POST', { request_id: pending.requestID });
        wx.removeStorageSync(NOTICE_PENDING_KEY);
      } catch (error) {
        this.setData({ notificationStatus: '微信已授权，但同步未完成，重新打开页面后会再试' });
      }
    }
    try {
      const state = await api.request('/api/player/notifications');
      this.setData({ notificationReady: true, notificationTemplateID: state.template_id, notificationRemaining: state.remaining, notificationHint: state.remaining > 0 ? `还可接收 ${state.remaining} 条` : '订阅下一条活动' });
    } catch (_) {
      this.setData({ notificationReady: false, notificationTemplateID: '', notificationHint: '暂未开放' });
    }
  },
  async subscribeNextEvent() {
    if (this.data.notificationBusy || !this.data.notificationTemplateID) return;
    const templateID = this.data.notificationTemplateID;
    this.setData({ notificationBusy: true, notificationStatus: '' });
    try {
      const result = await new Promise((resolve, reject) => wx.requestSubscribeMessage({ tmplIds: [templateID], success: resolve, fail: reject }));
      if (result[templateID] !== 'accept') {
        this.setData({ notificationStatus: result[templateID] === 'ban' ? '微信已限制此类提醒，请检查小程序通知设置' : '未授权新活动提醒' });
        return;
      }
      const requestID = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
      wx.setStorageSync(NOTICE_PENDING_KEY, { playerID: this.data.player.id, requestID });
      const state = await api.request('/api/player/notifications', 'POST', { request_id: requestID });
      wx.removeStorageSync(NOTICE_PENDING_KEY);
      this.setData({ notificationRemaining: state.remaining, notificationHint: `还可接收 ${state.remaining} 条`, notificationStatus: '已订阅下一条新活动提醒' });
    } catch (error) {
      this.setData({ notificationStatus: wx.getStorageSync(NOTICE_PENDING_KEY) ? '微信已授权，但同步未完成，重新打开页面后会再试' : (error.errMsg || error.message || '订阅失败，请重试') });
    } finally {
      this.setData({ notificationBusy: false });
    }
  },
  async load() {
    this.setData({ loading: true, error: '' });
    try {
      const player = await api.refreshMe();
      this.setData({ player, initial: player.name ? player.name.slice(0, 1) : '我' });
      if (player.has_avatar) this.loadAvatar(player.id);
      else this.setData({ avatarPath: '' });
      const result = await api.request('/api/player/stats/me');
      const eventResult = await api.request('/api/player/events');
      const byID = Object.fromEntries((eventResult.events || []).map(event => [event.id, event]));
      const history = result.history.map(row => ({
        ...row, ...view.eventDateParts(row.starts_at),
        timeText: view.dateText(row.starts_at), typeLabel: view.types[row.type],
        location: (byID[row.event_id] || {}).location || '地点待定',
        typeIcon: eventDesign(row.type).icon,
        ended: new Date(row.ends_at).getTime() <= Date.now(),
        rsvpText: (view.statuses.find(s => s.value === row.rsvp_status) || {}).label || '未确认'
      })).sort((a, b) => new Date(b.starts_at) - new Date(a.starts_at));
      const participationCount = view.participationCount(history);
      this.setData({ history, recentHistory: history.slice(0, 3), participationCount });
    } catch (e) { view.fail(this, e); }
    finally { this.setData({ loading: false }); wx.stopPullDownRefresh(); }
  },
  edit() {
    setTabBarHidden(this, true);
    this.setData({ editing: true, error: '' });
  },
  closeEdit() {
    if (this.data.saving) return;
    this.setData({ editing: false, error: '' });
    getApp().globalData.postProfileRoute = '';
    setTabBarHidden(this, false);
  },
  onHide() { this.avatarReviewActive = false; if (this.data.editing) setTabBarHidden(this, false); },
  stop() {},
  async loadAvatar(id) { try { const avatarPath = await api.avatar(id); if (this.data.player.id === id) this.setData({ avatarPath }); } catch (_) {} },
  async chooseAvatar() {
    if (this.data.uploadingAvatar) return;
    let file;
    try {
      const picked = await new Promise((resolve, reject) => wx.chooseMedia({ count: 1, mediaType: ['image'], sourceType: ['album', 'camera'], success: resolve, fail: reject }));
      file = picked.tempFiles && picked.tempFiles[0];
    } catch (error) {
      if (!String(error.errMsg || error.message || '').includes('cancel')) this.setData({ avatarStatus: '无法选择图片，请重试' });
      return;
    }
    if (!file || !file.tempFilePath) return;
    if (!file.size || file.size > AVATAR_MAX_BYTES) { this.setData({ avatarStatus: '请选择不超过 6 MB 的 JPEG、PNG 或 WebP 图片' }); return; }
    this.setData({ uploadingAvatar: true, avatarStatus: '正在上传并检查图片…' });
    try {
      const review = await api.uploadAvatar(file.tempFilePath);
      wx.setStorageSync(AVATAR_REVIEW_KEY, review.id);
      this.setData({ avatarStatus: '头像审核中，通过后会自动更新' });
      this.pollAvatarReview(review.id);
    } catch (error) {
      this.setData({ avatarStatus: error.message || '头像上传失败，请重试' });
    } finally {
      this.setData({ uploadingAvatar: false });
    }
  },
  resumeAvatarReview() {
    const job = wx.getStorageSync(AVATAR_REVIEW_KEY);
    if (job) this.pollAvatarReview(job);
  },
  async pollAvatarReview(job) {
    this.avatarReviewSeq = (this.avatarReviewSeq || 0) + 1;
    const seq = this.avatarReviewSeq;
    for (let attempt = 0; attempt < 15 && this.avatarReviewActive && seq === this.avatarReviewSeq; attempt++) {
      try {
        const result = await api.request('/api/player/reviews/' + encodeURIComponent(job));
        if (!this.avatarReviewActive || seq !== this.avatarReviewSeq) return;
        const review = result.review;
        if (review.status !== 'pending') {
          wx.removeStorageSync(AVATAR_REVIEW_KEY);
          if (review.status === 'approved') {
            const player = await api.refreshMe();
            api.clearAvatarCache(player.id);
            this.setData({ player, avatarStatus: '头像已更新' });
            this.loadAvatar(player.id);
          } else this.setData({ avatarStatus: review.message || '头像审核未通过，请重新选择' });
          return;
        }
        this.setData({ avatarStatus: '头像审核中，通过后会自动更新' });
      } catch (error) {
        this.setData({ avatarStatus: error.message || '暂时无法查询头像审核结果' });
        return;
      }
      await new Promise(resolve => setTimeout(resolve, 2000));
    }
  },
  async saveProfile(e) {
    if (this.data.saving) return;
    const name = (e.detail.value.nickname || '').trim();
    if (!name) { this.setData({ error: '请输入昵称' }); return; }
    this.setData({ saving: true, error: '' });
    try {
      const player = await api.profile(name);
      this.setData({ player, initial: player.name.slice(0, 1), avatarPath: this.data.avatarPath, editing: false });
      setTabBarHidden(this, false);
      view.notice('资料已保存');
      const route = getApp().globalData.postProfileRoute;
      getApp().globalData.postProfileRoute = '';
      if (route) wx.navigateTo({ url: route });
    } catch (e) { view.fail(this, e); }
    finally { this.setData({ saving: false }); }
  },
  settings() { wx.navigateTo({ url: '/pages/settings/index' }); },
  topics() { wx.navigateTo({ url: '/pages/topics/index' }); },
  events() { wx.switchTab({ url: '/pages/events/index' }); },
  member() { if (this.data.player.id) wx.navigateTo({ url: '/pages/member/index?id=' + encodeURIComponent(this.data.player.id) }); },
  open(e) { wx.navigateTo({ url: '/pages/event-detail/index?id=' + encodeURIComponent(e.currentTarget.dataset.id) }); },
  onShareAppMessage() { return { title: '邀请你加入吸土城FC', path: '/pages/login/index', imageUrl: share.teamImage }; },
  onPullDownRefresh() { this.load(); }
});
