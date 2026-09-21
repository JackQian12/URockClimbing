const path = require('node:path')
const automator = require('miniprogram-automator')

const projectPath = path.resolve(__dirname, '..')

async function main() {
  const miniProgram = await automator.launch({
    cliPath: '/Applications/wechatwebdevtools.app/Contents/MacOS/cli',
    projectPath,
    timeout: 120000,
  })

  try {
    const page = await miniProgram.reLaunch('/pages/home/index')
    await page.waitFor(1500)

    const greeting = await page.$('.greeting')
    if (!greeting || !(await greeting.text()).includes('岩友')) throw new Error('首页问候语未正确渲染')
    const loginTitle = await page.$('.login-title')
    const balance = await page.$('.balance-number')
    if (!loginTitle && !balance) throw new Error('首页会员状态未正确渲染')
    if (loginTitle && (await loginTitle.text()) !== '请先登录' && (await loginTitle.text()) !== '请授权手机') {
      throw new Error('首页登录提示未正确渲染')
    }
    if (balance) {
      const checkinEntry = await page.$('.checkin-entry')
      if (!checkinEntry) throw new Error('未找到会员扫码签到入口')
    }

    console.log(
      JSON.stringify(
        {
          result: 'passed',
          checks: ['home-render', 'greeting-render', 'member-state-render', 'registered-checkin-entry'],
        },
        null,
        2,
      ),
    )
  } finally {
    miniProgram.disconnect()
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
