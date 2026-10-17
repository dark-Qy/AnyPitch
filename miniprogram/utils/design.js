// Presentation metadata only. Event membership, permissions and counts come from the API.
const eventTypes = {
  training: { label: '训练', icon: '/assets/activities/training.svg', tone: 'training' },
  friendly: { label: '友谊赛', icon: '/assets/activities/friendly.svg', tone: 'friendly' },
  pickup: { label: '野球', icon: '/assets/activities/pickup.svg', tone: 'pickup' }
};
const typeChoices = [{ value: 'all', label: '全部', icon: '/assets/icons/grid-dark.svg' },
  ...Object.entries(eventTypes).map(([value, item]) => ({ value, ...item }))];
function eventDesign(type) {
  return eventTypes[type] || { label: '活动', icon: '/assets/icons/calendar-dark.svg', tone: 'neutral' };
}
module.exports = { eventTypes, typeChoices, eventDesign };
