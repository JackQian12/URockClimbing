import { confirmCheckin, createRedemptionRequestID, previewCheckin } from '../../services/redemptions'
import type { CheckinCard, CheckinPreview, CheckinResult } from '../../types/api'

type DisplayCard = CheckinCard & { benefit: string; hint: string }

Page({
  data: {
    scanning: false,
    loading: false,
    confirming: false,
    token: '',
    preview: null as (CheckinPreview & { cards: DisplayCard[] }) | null,
    selectedCardID: '',
    result: null as CheckinResult | null,
    error: '',
    requestID: '',
  },

  handleScan() {
    if (this.data.scanning || this.data.confirming) return
    this.setData({ scanning: true, error: '', result: null })
    wx.scanCode({
      onlyFromCamera: true,
      scanType: ['qrCode'],
      success: ({ result }) => { void this.loadCode(result) },
      fail: (error) => {
        if (!String(error.errMsg || '').includes('cancel')) this.setData({ error: '扫码失败，请对准场馆签到码重试' })
      },
      complete: () => this.setData({ scanning: false }),
    })
  },

  async loadCode(token: string) {
    this.setData({ loading: true, token, preview: null, selectedCardID: '', requestID: '', error: '' })
    try {
      const preview = await previewCheckin(token)
      const cards: DisplayCard[] = preview.cards.map(card => ({
        ...card,
        benefit: card.product_type === 'COUNT_CARD' ? `剩余 ${card.remaining_times ?? 0} 次` : '期限内畅爬',
        hint: card.status === 'PENDING_ACTIVATION' ? '本次签到后激活' : card.expires_at ? `有效至 ${new Date(card.expires_at).toLocaleDateString('zh-CN')}` : '可使用',
      }))
      this.setData({ preview: { ...preview, cards }, selectedCardID: cards.length === 1 ? cards[0].card_id : '' })
    } catch (error) {
      this.setData({ token: '', error: error instanceof Error ? error.message : '签到码校验失败' })
    } finally {
      this.setData({ loading: false })
    }
  },

  handleSelect(event: WechatMiniprogram.TouchEvent) {
    const cardID = String(event.currentTarget.dataset.id || '')
    if (cardID) this.setData({ selectedCardID: cardID, requestID: '', error: '' })
  },

  async handleConfirm() {
    if (this.data.confirming || !this.data.preview || !this.data.selectedCardID || !this.data.token) return
    const card = this.data.preview.cards.find(item => item.card_id === this.data.selectedCardID) as DisplayCard | undefined
    if (!card) return
    const modal = await wx.showModal({
      title: card.product_type === 'COUNT_CARD' ? '确认核销 1 次？' : '确认本日签到？',
      content: `${card.product_name}\n${card.product_type === 'COUNT_CARD' ? card.benefit : '期限卡不扣减次数'}`,
      confirmText: '确认',
      confirmColor: '#5a2d16',
    })
    if (!modal.confirm) return
    const requestID = this.data.requestID || createRedemptionRequestID()
    this.setData({ confirming: true, requestID, error: '' })
    try {
      const result = await confirmCheckin(this.data.token, card.card_id, requestID)
      this.setData({ result, preview: null, token: '', requestID: '' })
      wx.showToast({ title: card.product_type === 'COUNT_CARD' ? '核销成功' : '签到成功', icon: 'success' })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '操作失败，请重试' })
    } finally {
      this.setData({ confirming: false })
    }
  },

  handleDone() { wx.navigateBack({ delta: 1 }) },
})
