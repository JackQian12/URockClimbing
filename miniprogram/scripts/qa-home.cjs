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

    const balance = await page.$('.balance-number')
    if (!balance || (await balance.text()) !== '7') {
      throw new Error('首页会员次数未正确渲染')
    }
    const buyButton = await page.$('.buy-link')
    if (!buyButton) throw new Error('未找到“购买次卡”入口')
    const redeemButton = await page.$('.redeem-button')
    if (!redeemButton) throw new Error('未找到“生成核销码”按钮')

    console.log(
      JSON.stringify(
        {
          result: 'passed',
          checks: ['home-render', 'balance-render', 'buy-card-entry', 'redeem-entry'],
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
