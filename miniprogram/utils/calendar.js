const weekLabels = ['一', '二', '三', '四', '五', '六', '日'];

function dateKey(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Date(date.getTime() + 8 * 3600000).toISOString().slice(0, 10);
}

function monthGrid(month, events, selectedDate = '') {
  const match = /^(\d{4})-(0[1-9]|1[0-2])$/.exec(month);
  if (!match) return [];
  const year = Number(match[1]);
  const monthIndex = Number(match[2]) - 1;
  const first = new Date(Date.UTC(year, monthIndex, 1));
  const offset = (first.getUTCDay() + 6) % 7;
  const days = new Date(Date.UTC(year, monthIndex + 1, 0)).getUTCDate();
  const byDate = new Set();
  for (const event of events || []) {
    const key = dateKey(event.starts_at);
    if (!key.startsWith(month + '-')) continue;
    byDate.add(key);
  }
  const today = dateKey(new Date().toISOString());
  return Array.from({ length: Math.ceil((offset + days) / 7) * 7 }, (_, index) => {
    const day = index - offset + 1;
    if (day < 1 || day > days) return { key: `blank-${index}`, blank: true };
    const key = `${month}-${String(day).padStart(2, '0')}`;
    return { key, day, hasEvent: byDate.has(key), today: key === today, selected: key === selectedDate };
  });
}

function monthWeeks(month, events, selectedDate = '') {
  const days = monthGrid(month, events, selectedDate);
  return Array.from({ length: days.length / 7 }, (_, index) => ({ key: index, days: days.slice(index * 7, (index + 1) * 7) }));
}

function shiftMonth(month, delta) {
  const [year, number] = month.split('-').map(Number);
  const date = new Date(Date.UTC(year, number - 1 + delta, 1));
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
}

function monthLabel(month) { const [year, number] = month.split('-'); return `${year} 年 ${Number(number)} 月`; }
module.exports = { weekLabels, dateKey, monthGrid, monthWeeks, shiftMonth, monthLabel };
