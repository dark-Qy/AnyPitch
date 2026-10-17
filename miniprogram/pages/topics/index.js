const api = require('../../utils/api');
const view = require('../../utils/view');

Page({
  onReady() { getApp().initTitleFont(); },
  data: { loading: true, error: '', topics: [] },
  onShow() { if (view.requireSession()) this.load(); },
  async load() {
    this.setData({ loading: true, error: '' });
    try {
      const result = await api.request('/api/player/special-activities');
      const topics = await Promise.all(result.activities.map(async activity => {
        const detail = await api.request('/api/player/special-activities/' + encodeURIComponent(activity.id) + '/entries');
        return { ...activity, entries: detail.entries.map(entry => ({ ...entry, timeText: view.dateText(entry.occurs_at), scoreText: entry.home_score === null || entry.away_score === null ? '比分待定' : `${entry.home_score} : ${entry.away_score}` })) };
      }));
      this.setData({ topics });
    } catch (error) { view.fail(this, error); }
    finally { this.setData({ loading: false }); wx.stopPullDownRefresh(); }
  },
  onPullDownRefresh() { this.load(); }
});
