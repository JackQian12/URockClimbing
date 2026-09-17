import drawQrcode from 'weapp-qrcode'
import { createRedemptionToken } from '../../services/redemptions'

Page({
  data: { cardId: '', loading: true, error: '', secondsLeft: 0, expiresLabel: '' },
  timer: 0 as number,
  onLoad(options: Record<string,string|undefined>) { this.setData({ cardId: options.card_id || '' }); void this.refreshToken() },
  onUnload() { this.stopTimer() },
  async refreshToken() {
    if (!this.data.cardId) { this.setData({ loading: false, error: '会员卡无效' }); return }
    this.stopTimer(); this.setData({ loading: true, error: '' })
    try {
      const result = await createRedemptionToken(this.data.cardId)
      const expiresAt = new Date(result.expires_at).getTime()
      this.setData({ loading: false, expiresLabel: new Date(result.expires_at).toLocaleTimeString('zh-CN') })
      setTimeout(() => drawQrcode({ width: 240, height: 240, canvasId: 'redemptionQrcode', text: result.token, background: '#ffffff', foreground: '#2f1b12', _this: this }), 80)
      this.updateCountdown(expiresAt)
      this.timer = setInterval(() => this.updateCountdown(expiresAt), 1000) as unknown as number
    } catch (error) { this.setData({ loading: false, error: error instanceof Error ? error.message : '核销码生成失败' }) }
  },
  updateCountdown(expiresAt: number) { const seconds = Math.max(0, Math.ceil((expiresAt - Date.now()) / 1000)); this.setData({ secondsLeft: seconds }); if (seconds === 0) this.stopTimer() },
  stopTimer() { if (this.timer) clearInterval(this.timer); this.timer = 0 },
  handleRefresh() { void this.refreshToken() },
})
