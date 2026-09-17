declare module 'weapp-qrcode' {
  type Options = { width: number; height: number; canvasId?: string; ctx?: WechatMiniprogram.CanvasContext; text: string; background?: string; foreground?: string; _this?: WechatMiniprogram.Page.Instance<WechatMiniprogram.IAnyObject, WechatMiniprogram.IAnyObject> }
  export default function drawQrcode(options: Options): void
}
