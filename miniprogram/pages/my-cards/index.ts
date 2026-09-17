import { listMyCards } from '../../services/cards'
import type { MemberCard, MemberCardStatus } from '../../types/api'
const labels: Record<MemberCardStatus,string> = { PENDING_ACTIVATION:'待激活', ACTIVE:'可使用', USED_UP:'已用完', EXPIRED:'已过期', REFUND_LOCKED:'退款中', REFUNDED:'已退款' }
type DisplayCard = MemberCard & { statusLabel:string; benefit:string; expiryLabel:string }
Page({
  data:{items:[] as DisplayCard[],loading:true,error:''},
  onLoad(){void this.loadCards()}, onShow(){if(!this.data.loading)void this.loadCards()},
  onPullDownRefresh(){void this.loadCards().finally(()=>wx.stopPullDownRefresh())},
  async loadCards(){this.setData({loading:true,error:''});try{const r=await listMyCards();this.setData({items:r.items.map(i=>({...i,statusLabel:labels[i.status],benefit:i.product_type==='COUNT_CARD'?`剩余 ${i.remaining_times??0} / ${i.total_times??0} 次`:`${i.validity_days} 天畅爬`,expiryLabel:i.status==='PENDING_ACTIVATION'?'首次使用后开始计时':i.expires_at?`有效至 ${new Date(i.expires_at).toLocaleString('zh-CN')}`:'—'}))})}catch(e){this.setData({error:e instanceof Error?e.message:'会员卡加载失败'})}finally{this.setData({loading:false})}},
  handleCard(e:WechatMiniprogram.BaseEvent){const id=String(e.currentTarget.dataset.id||'');if(id)wx.navigateTo({url:`/pages/my-cards/detail?id=${encodeURIComponent(id)}`})},
  handleStore(){wx.switchTab({url:'/pages/cards/index'})},
})
