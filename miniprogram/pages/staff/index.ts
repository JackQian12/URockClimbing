import { listTodayRedemptions, setPendingScannedToken } from '../../services/redemptions'
import type { StaffRedemption } from '../../types/api'
Page({
 data:{items:[] as Array<StaffRedemption&{timeLabel:string;changeLabel:string}>,loading:true,error:''},
 onLoad(){void this.load()},onShow(){if(!this.data.loading)void this.load()},onPullDownRefresh(){void this.load().finally(()=>wx.stopPullDownRefresh())},
 async load(){this.setData({loading:true,error:''});try{const r=await listTodayRedemptions();this.setData({items:r.items.map(i=>({...i,timeLabel:new Date(i.redeemed_at).toLocaleTimeString('zh-CN'),changeLabel:i.after_remaining===null?'期限卡使用':`${i.before_remaining} → ${i.after_remaining} 次`}))})}catch(e){this.setData({error:e instanceof Error?e.message:'今日记录加载失败'})}finally{this.setData({loading:false})}},
 async handleScan(){try{const result=await wx.scanCode({scanType:['qrCode']});const token=String(result.result||'').trim();if(!token){wx.showToast({title:'核销码无效',icon:'none'});return}setPendingScannedToken(token);wx.navigateTo({url:'/pages/staff/redeem'})}catch{/* user cancelled */}},
})
