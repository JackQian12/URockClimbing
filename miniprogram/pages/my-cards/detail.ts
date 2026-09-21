import { getMyCard, listCardRedemptions } from '../../services/cards'
import type { MemberCard, MemberRedemption } from '../../types/api'
const labels:Record<string,string>={PENDING_ACTIVATION:'待激活',ACTIVE:'可使用',USED_UP:'已用完',EXPIRED:'已过期',REFUND_LOCKED:'退款中',REFUNDED:'已退款'}
Page({
 data:{id:'',card:null as MemberCard|null,records:[] as Array<MemberRedemption&{timeLabel:string;changeLabel:string}>,statusLabel:'',benefit:'',expiryLabel:'',loading:true,error:''},
 onLoad(o:Record<string,string|undefined>){this.setData({id:o.id||''});void this.load()},onShow(){if(this.data.id&&!this.data.loading)void this.load()},onPullDownRefresh(){void this.load().finally(()=>wx.stopPullDownRefresh())},
 async load(){if(!this.data.id){this.setData({loading:false,error:'会员卡无效'});return}this.setData({loading:true,error:''});try{const[c,r]=await Promise.all([getMyCard(this.data.id),listCardRedemptions(this.data.id)]);this.setData({card:c,statusLabel:labels[c.status]||c.status,benefit:c.product_type==='COUNT_CARD'?`${c.remaining_times??0} / ${c.total_times??0} 次`:`${c.validity_days} 天畅爬`,expiryLabel:c.expires_at?new Date(c.expires_at).toLocaleString('zh-CN'):'激活后计算',records:r.items.map(i=>({...i,timeLabel:new Date(i.redeemed_at).toLocaleString('zh-CN'),changeLabel:i.after_remaining===null?'期限卡当日使用':`${i.before_remaining} → ${i.after_remaining} 次`}))})}catch(e){this.setData({error:e instanceof Error?e.message:'会员卡加载失败'})}finally{this.setData({loading:false})}},
 handleRedeem(){wx.navigateTo({url:`/pages/redemption-code/index?card_id=${encodeURIComponent(this.data.id)}`})},
 handleCheckin(){wx.navigateTo({url:'/pages/checkin/index'})},
})
