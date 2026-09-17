import { confirmRedemption, createRedemptionRequestID, previewRedemption, takePendingScannedToken } from '../../services/redemptions'
import type { RedemptionPreview, RedemptionResult } from '../../types/api'
Page({
 data:{token:'',preview:null as RedemptionPreview|null,result:null as RedemptionResult|null,memberInitial:'岩',benefit:'',expiry:'',loading:true,confirming:false,error:''},
 onLoad(){const token=takePendingScannedToken();this.setData({token});void this.loadPreview()},
 onUnload(){this.setData({token:''})},
 async loadPreview(){if(!this.data.token){this.setData({loading:false,error:'未获取到核销码，请重新扫码'});return}try{const p=await previewRedemption(this.data.token);this.setData({preview:p,memberInitial:(p.nickname||'岩').slice(0,1),benefit:p.product_type==='COUNT_CARD'?`剩余 ${p.remaining_times??0} 次`:(p.status==='PENDING_ACTIVATION'?'首次使用将激活':'期限内不限次'),expiry:p.expires_at?new Date(p.expires_at).toLocaleString('zh-CN'):'核销后计算'})}catch(e){this.setData({error:e instanceof Error?e.message:'核销码校验失败'})}finally{this.setData({loading:false})}},
 async handleConfirm(){if(this.data.confirming||!this.data.preview)return;const modal=await wx.showModal({title:'确认核销',content:`确认使用「${this.data.preview.product_name}」？核销成功后不可撤销。`,confirmText:'确认核销',confirmColor:'#8a3f20'});if(!modal.confirm)return;this.setData({confirming:true,error:''});try{const result=await confirmRedemption(this.data.token,createRedemptionRequestID());this.setData({result,token:''});wx.showToast({title:'核销成功',icon:'success'})}catch(e){this.setData({error:e instanceof Error?e.message:'核销失败'})}finally{this.setData({confirming:false})}},
 handleBack(){wx.navigateBack()},
})
