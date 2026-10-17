const FAMILY = 'AnyPitch Serif';
const URL = 'https://pitch.coolcookie.cloud/assets/miniprogram/fonts/source-han-serif-medium.woff';
function loadTitleFont() {
  return new Promise((resolve, reject) => wx.loadFontFace({
    global: true, family: FAMILY, source: `url("${URL}")`,
    success: resolve, fail: reject
  }));
}
module.exports = { FAMILY, URL, loadTitleFont };
