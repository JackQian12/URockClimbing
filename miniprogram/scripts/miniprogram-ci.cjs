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
  if (!['preview', 'upload'].includes(action)) {
    throw new Error('Usage: npm run preview|upload -- --version=x.y.z --desc="description"')
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

  const version = readOption('version')
  const desc = readOption('desc')
  if (!version || !desc) {
    throw new Error('Upload requires --version and --desc')
  }
  await ci.upload({ ...common, version, desc })
}

main().catch((error) => {
  console.error(error.message || error)
  process.exitCode = 1
})
