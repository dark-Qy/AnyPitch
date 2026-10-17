const api=require('../../utils/api');
Page({
 onReady() { getApp().initTitleFont(); },
 data:{busy:false,error:''},
 enter(){const data=getApp().globalData;const route=data.postLoginRoute;data.postLoginRoute='';if(route)wx.reLaunch({url:route});else wx.switchTab({url:'/pages/events/index'});},
 onShow(){if(api.session())this.enter();else this.submit();},
 async submit(){if(this.data.busy)return;this.setData({busy:true,error:''});try{await api.login();this.enter();}catch(e){this.setData({error:e.message});}finally{this.setData({busy:false});}},
 onPullDownRefresh(){wx.stopPullDownRefresh();}
});
