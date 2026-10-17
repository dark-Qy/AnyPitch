const BASE_URL = 'https://pitch.coolcookie.cloud';
const SESSION_KEY = 'anypitch_wechat_session';
function session() { return wx.getStorageSync(SESSION_KEY) || null; }
function clearSession() { wx.removeStorageSync(SESSION_KEY); }
function headers() {
  const current = session();
  return current ? { Authorization: `Bearer ${current.token}` } : {};
}
function decode(response) {
  if (response.statusCode === 401) {
    clearSession();
    const error = new Error('登录已失效，请重新进入'); error.status = 401; throw error;
  }
  let payload = response.data;
  if (typeof payload === 'string') {
    try { payload = JSON.parse(payload); } catch (_) { throw new Error('服务返回异常，请稍后重试'); }
  }
  if (response.statusCode < 200 || response.statusCode >= 300 || (payload && payload.error)) {
    const code = payload && payload.error && payload.error.code;
    const messages = { invalid_credentials: '微信登录凭证已失效，请重试', event_cancelled: '活动已取消，不能报名' };
    const error=new Error(messages[code] || (payload && payload.error && payload.error.message) || '请求失败，请稍后重试');error.status=response.statusCode;throw error;
  }
  if (!payload || payload.data === undefined) throw new Error('服务版本不匹配，请联系教练');
  return payload.data;
}
function request(path, method = 'GET', data) {
  return new Promise((resolve, reject) => wx.request({
    url: BASE_URL + path, method, data, timeout: 15000,
    header: { 'content-type': 'application/json', ...headers() },
    success(res) { try { resolve(decode(res)); } catch (error) { reject(error); } },
    fail() { reject(new Error('网络连接失败，请检查网络后重试')); }
  }));
}
async function login() {
  const code = await new Promise((resolve,reject)=>wx.login({timeout:10000,success:r=>r.code?resolve(r.code):reject(new Error('微信登录失败，请重试')),fail:()=>reject(new Error('微信登录失败，请重试'))}));
  const result=await request('/api/auth/wechat','POST',{code});
  wx.setStorageSync(SESSION_KEY,result);return result;
}
async function profile(name) {
  const result=await request('/api/player/profile','PATCH',{name});
  const current=session();wx.setStorageSync(SESSION_KEY,{...current,player:result.player});return result.player;
}
async function refreshMe() {
  const result=await request('/api/player/me');
  const current=session();if(current)wx.setStorageSync(SESSION_KEY,{...current,player:result.player});return result.player;
}
async function logout() {
  try { await request('/api/player/logout', 'POST'); } finally { clearSession(); }
}
const avatarCache = {};
function avatar(playerID) {
  if (avatarCache[playerID]) return Promise.resolve(avatarCache[playerID]);
  return new Promise((resolve, reject) => wx.downloadFile({
    url: `${BASE_URL}/api/player/avatars/${encodeURIComponent(playerID)}`,
    header: headers(), timeout: 30000,
    success(res) { if (res.statusCode === 200) { avatarCache[playerID] = res.tempFilePath; resolve(res.tempFilePath); } else reject(new Error('头像加载失败')); },
    fail() { reject(new Error('头像加载失败')); }
  }));
}
function clearAvatarCache(playerID) { delete avatarCache[playerID]; }
function uploadAvatar(filePath) {
  return new Promise((resolve, reject) => wx.uploadFile({
    url: `${BASE_URL}/api/player/avatar`, filePath, name: 'file', header: headers(), timeout: 30000,
    success(res) { try { resolve(decode(res).review); } catch (error) { reject(error); } },
    fail() { reject(new Error('头像上传失败，请检查网络后重试')); }
  }));
}
module.exports = { BASE_URL, session, headers, request, login, profile, refreshMe, logout, avatar, clearAvatarCache, uploadAvatar, clearSession };
