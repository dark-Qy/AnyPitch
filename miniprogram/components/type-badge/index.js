const { eventDesign } = require('../../utils/design');
Component({ properties: { type: { type: String, value: '', observer(type) { this.setData({ design: eventDesign(type) }); } } }, data: { design: eventDesign('') } });
