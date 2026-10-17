const { loadTitleFont } = require('./utils/fonts');
App({
  globalData: { editProfileOnShow: false, postProfileRoute: '', titleFontLoaded: false },
  onLaunch() {
    const updateManager = wx.getUpdateManager();
    updateManager.onUpdateReady(() => wx.showModal({
      title: '发现新版本', content: '新版本已准备好，重启后即可使用。',
      success: result => { if (result.confirm) updateManager.applyUpdate(); }
    }));
  },
  initTitleFont() {
    if (this.globalData.titleFontLoaded) return;
    return loadTitleFont().then(() => { this.globalData.titleFontLoaded = true; delete this.globalData.titleFontError; })
      .catch(error => { this.globalData.titleFontError = error.errMsg || error.message || 'font load failed'; console.warn('AnyPitch title font could not be loaded'); });
  }
});
