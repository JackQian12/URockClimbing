const fs = require('node:fs')
const path = require('node:path')
const ci = require('miniprogram-ci')

const appid = 'wxfa1a6db20e6026d8'
const projectPath = path.resolve(__dirname, '..')
const privateKeyPath = path.join(
  projectPath,
  '.secrets',
  'private.wxfa1a6db20e6026d8.key',
)

function readOption(name, fallback = '') {
  const prefix = `--${name}=`
  const argument = process.argv.find((item) => item.startsWith(prefix))
  return argument ? argument.slice(prefix.length) : fallback
}

async function main() {
  const action = process.argv[2]
  if (action !== 'preview') {
    throw new Error(
      'CI upload is disabled because it publishes as CI机器人1. Use npm run upload with WeChat DevTools logged in as Jack.',
    )
  }

  const project = new ci.Project({
    appid,
    type: 'miniProgram',
    projectPath,
    privateKeyPath,
    ignores: ['node_modules/**/*', '.secrets/**/*', 'qa/**/*'],
  })

  const common = {
    project,
    setting: {
      es6: true,
      es7: true,
      minify: true,
      codeProtect: true,
      autoPrefixWXSS: true,
    },
    onProgressUpdate: console.log,
  }

  if (action === 'preview') {
    const outputDir = path.join(projectPath, '.miniprogram-ci')
    const qrcodeOutputDest = path.join(outputDir, 'preview.jpg')
    fs.mkdirSync(outputDir, { recursive: true })
    await ci.preview({
      ...common,
      desc: readOption('desc', 'URock preview'),
      qrcodeFormat: 'image',
      qrcodeOutputDest,
    })
    console.log(`Preview QR code: ${qrcodeOutputDest}`)
    return
  }
}

main().catch((error) => {
  console.error(error.message || error)
  process.exitCode = 1
})
