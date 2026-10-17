const tabs = [
  { pagePath: '/pages/events/index', text: '活动', icon: '/assets/icons/calendar-dark.svg', activeIcon: '/assets/icons/calendar-active.svg' },
  { pagePath: '/pages/rsvp/index', text: '报名', icon: '/assets/icons/check-calendar-dark.svg', activeIcon: '/assets/icons/check-calendar-active.svg' },
  { pagePath: '/pages/stats/index', text: '训练榜', icon: '/assets/icons/trophy-dark.svg', activeIcon: '/assets/icons/trophy-active.svg' },
  { pagePath: '/pages/mine/index', text: '我的', icon: '/assets/icons/user-round-dark.svg', activeIcon: '/assets/icons/user-round-active.svg' }
];

Component({
  data: { selected: 0, hidden: false, tabs },
  lifetimes: {
    attached() {
      const current = getCurrentPages().slice(-1)[0];
      const selected = tabs.findIndex(tab => tab.pagePath === '/' + (current && current.route));
      if (selected >= 0) this.setData({ selected });
    }
  },
  methods: {
    switchPage(e) {
      const index = Number(e.currentTarget.dataset.index);
      if (index === this.data.selected) return;
      wx.switchTab({ url: tabs[index].pagePath });
    }
  }
});
