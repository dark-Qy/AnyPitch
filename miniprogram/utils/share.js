const view = require('./view');

const teamImage = '/assets/share/invite-team.jpg';
const eventBaseImage = '/assets/share/invite-event-base.jpg';

function eventPath(id) { return '/pages/event-detail/index?id=' + encodeURIComponent(id); }
function eventTime(event) {
  const start = view.dateText(event.starts_at);
  const end = view.dateText(event.ends_at);
  return start.split(' ')[0] === end.split(' ')[0] ? start + '—' + end.split(' ')[1] : start + '—' + end;
}
function fit(ctx, value, width) {
  let text = String(value || '');
  while (text && ctx.measureText(text).width > width) text = text.slice(0, -1);
  return text === value ? text : text.slice(0, -1) + '…';
}
function drawEventImage(page, event) {
  return new Promise((resolve, reject) => {
    const ctx = wx.createCanvasContext('event-share-canvas', page);
    ctx.drawImage(eventBaseImage, 0, 0, 1402, 1122);
    ctx.setFillStyle('#e3effa'); ctx.beginPath(); ctx.arc(190, 278, 48, Math.PI / 2, Math.PI * 1.5); ctx.arc(276, 278, 48, Math.PI * 1.5, Math.PI * 2.5); ctx.closePath(); ctx.fill();
    ctx.setFillStyle('#173d64'); ctx.setFontSize(39); ctx.fillText(view.types[event.type] || '活动', 185, 292);
    ctx.setFillStyle('#102f50'); ctx.setFontSize(82); ctx.fillText(fit(ctx, event.title, 700), 150, 455);
    ctx.setFillStyle('#6b9bd0'); ctx.setLineWidth(5); ctx.strokeRect(158, 529, 48, 48); ctx.fillRect(168, 520, 5, 15); ctx.fillRect(190, 520, 5, 15);
    ctx.beginPath(); ctx.arc(182, 665, 22, 0, Math.PI * 2); ctx.stroke(); ctx.beginPath(); ctx.arc(182, 665, 7, 0, Math.PI * 2); ctx.stroke();
    ctx.setFillStyle('#102f50'); ctx.setFontSize(46); ctx.fillText(fit(ctx, eventTime(event), 625), 255, 574);
    ctx.fillText(fit(ctx, event.location || '地点待定', 625), 255, 680);
    ctx.draw(false, () => wx.canvasToTempFilePath({
      canvasId: 'event-share-canvas', width: 1402, height: 1122, destWidth: 1402, destHeight: 1122,
      success: result => resolve(result.tempFilePath), fail: reject
    }, page));
  });
}

module.exports = { teamImage, eventPath, eventTime, drawEventImage };
