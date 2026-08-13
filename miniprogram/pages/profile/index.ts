import { getMe, loginWithWechat, logout, updateMe } from '../../services/auth'
import { tokenStore } from '../../store/token'
import type { MemberProfile } from '../../types/api'

Page({
  data: {
    profile: null as MemberProfile | null,
    nickname: '',
	profileInitial: '岩',
	roleLabel: '会员',
    loading: true,
    loggingIn: false,
    saving: false,
    error: '',
  },

  onShow() {
    void this.loadProfile()
  },

  onPullDownRefresh() {
    void this.loadProfile().finally(() => wx.stopPullDownRefresh())
  },

  async loadProfile() {
    this.setData({ loading: true, error: '' })
    try {
      const profile = await getMe()
		this.setData({
			profile,
			nickname: profile.nickname ?? '',
			profileInitial: (profile.nickname ?? '岩').slice(0, 1),
			roleLabel: profile.role === 'MEMBER' ? '会员' : profile.role === 'STAFF' ? '员工' : '管理员',
		})
    } catch (error) {
      this.setData({
        profile: null,
        error: error instanceof Error ? error.message : '会员资料加载失败',
      })
    } finally {
      this.setData({ loading: false, loggingIn: false })
    }
  },

  async handleLogin() {
    if (this.data.loggingIn) return
    this.setData({ loggingIn: true, error: '' })
    try {
      await loginWithWechat()
      await this.loadProfile()
      wx.showToast({ title: '登录成功', icon: 'success' })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '登录失败，请重试' })
    } finally {
      this.setData({ loggingIn: false })
    }
  },

  handleNicknameInput(event: WechatMiniprogram.Input) {
    this.setData({ nickname: event.detail.value })
  },

  async handleSave() {
    const profile = this.data.profile
    if (!profile || this.data.saving) return
    this.setData({ saving: true })
    try {
      const updated = await updateMe(this.data.nickname.trim(), profile.version)
		this.setData({ profile: updated, nickname: updated.nickname ?? '', profileInitial: (updated.nickname ?? '岩').slice(0, 1) })
      wx.showToast({ title: '已保存', icon: 'success' })
    } catch (error) {
      wx.showToast({ title: error instanceof Error ? error.message : '保存失败', icon: 'none' })
    } finally {
      this.setData({ saving: false })
    }
  },

  async handleLogout() {
    const result = await wx.showModal({ title: '退出登录', content: '退出后需重新使用微信登录。' })
    if (!result.confirm) return
    await logout()
    this.setData({ profile: null, nickname: '', error: '' })
  },

  isLoggedIn(): boolean {
    return Boolean(tokenStore.getAccessToken())
  },
})
