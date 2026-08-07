export function adminPath(): string {
  const path = window.location.pathname.replace(/^\/admin/, '') || '/'
  return path.endsWith('/') && path !== '/' ? path.slice(0, -1) : path
}

export function navigate(path: string, replace = false): void {
  const target = `/admin${path === '/' ? '/' : path}`
  window.history[replace ? 'replaceState' : 'pushState']({}, '', target)
  window.dispatchEvent(new PopStateEvent('popstate'))
}
