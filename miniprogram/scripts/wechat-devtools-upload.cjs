const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const projectPath = path.resolve(__dirname, '..')
const defaultCliPath = '/Applications/wechatwebdevtools.app/Contents/MacOS/cli'
const cliPath = process.env.WECHAT_DEVTOOLS_CLI || defaultCliPath

function readOption(name) {
  const prefix = `--${name}=`
  const argument = process.argv.find((item) => item.startsWith(prefix))
  return argument ? argument.slice(prefix.length) : ''
}

function runCli(args, captureOutput = false) {
  const result = spawnSync(cliPath, args, {
    encoding: 'utf8',
    stdio: captureOutput ? 'pipe' : 'inherit',
  })

  if (result.error) {
    throw result.error
  }
  if (result.status !== 0) {
    if (captureOutput) {
      process.stderr.write(result.stderr || result.stdout || '')
    }
    throw new Error(`WeChat DevTools CLI exited with status ${result.status}`)
  }
  return result
}

function main() {
  const version = readOption('version')
  const desc = readOption('desc')
  if (!version || !desc) {
    throw new Error(
      'Usage: npm run upload -- --version=x.y.z --desc="description"',
    )
  }
  if (!fs.existsSync(cliPath)) {
    throw new Error(
      `WeChat DevTools CLI not found at ${cliPath}. Set WECHAT_DEVTOOLS_CLI to its path.`,
    )
  }

  const login = runCli(
    ['islogin', '--project', projectPath, '--lang', 'zh'],
    true,
  )
  const loginOutput = `${login.stdout || ''}${login.stderr || ''}`
  if (!loginOutput.includes('"login":true')) {
    throw new Error(
      'WeChat DevTools is not logged in. Log in with the Jack developer account before uploading.',
    )
  }

  console.log(
    'Uploading through WeChat DevTools. Confirm the IDE is logged in as Jack; CI upload is disabled.',
  )
  const builtQrcodeModule = path.join(projectPath, 'miniprogram_npm', 'weapp-qrcode', 'index.js')
  if (!fs.existsSync(builtQrcodeModule)) {
    runCli(['build-npm', '--project', projectPath, '--lang', 'zh'])
  }
  runCli([
    'upload',
    '--project',
    projectPath,
    '--version',
    version,
    '--desc',
    desc,
    '--lang',
    'zh',
  ])
}

try {
  main()
} catch (error) {
  console.error(error.message || error)
  process.exitCode = 1
}
