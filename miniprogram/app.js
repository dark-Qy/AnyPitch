const { loadTitleFont } = require('./utils/fonts');
App({
  globalData: { editProfileOnShow: false, postProfileRoute: '', titleFontLoaded: false },
  initTitleFont() {
    if (this.globalData.titleFontLoaded) return;
    return loadTitleFont().then(() => { this.globalData.titleFontLoaded = true; delete this.globalData.titleFontError; })
      .catch(error => { this.globalData.titleFontError = error.errMsg || error.message || 'font load failed'; console.warn('AnyPitch title font could not be loaded'); });
  }
});
