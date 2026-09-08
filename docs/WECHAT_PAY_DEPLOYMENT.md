# WeChat Pay production activation

The payment and admin-only full-refund code is fail-closed. Keep `WECHAT_PAY_ENABLED=false` until every item below is installed and a low-value real-payment test has passed.

## Required merchant material

Obtain these values from the WeChat Pay merchant platform. Never send them through source control or place them in a Docker image.

- merchant API certificate serial number;
- merchant API private key (`apiclient_key.pem`);
- 32-byte API v3 key;
- WeChat Pay public key ID;
- WeChat Pay public key PEM.

The integration uses WeChat Pay's public-key verification mode, which is suitable for newly onboarded merchants. Payment and refund notifications are independently configured.

## Server files

Install the two PEM files on the production host:

```text
/opt/urock/deploy/secrets/wechatpay/apiclient_key.pem
/opt/urock/deploy/secrets/wechatpay/wechatpay_public_key.pem
```

Set the files to owner UID/GID `65532:65532`, mode `0400`. Docker mounts the directory read-only at `/run/secrets/wechatpay`.

Add the following to the production `.env` without printing the values to logs:

```dotenv
WECHAT_PAY_ENABLED=false
WECHAT_PAY_MCH_ID=<merchant-id>
WECHAT_PAY_API_V3_KEY=<exactly-32-bytes>
WECHAT_PAY_CERT_SERIAL_NO=<merchant-api-certificate-serial>
WECHAT_PAY_PRIVATE_KEY_PATH=/run/secrets/wechatpay/apiclient_key.pem
WECHAT_PAY_PUBLIC_KEY_ID=<wechat-pay-public-key-id>
WECHAT_PAY_PUBLIC_KEY_PATH=/run/secrets/wechatpay/wechatpay_public_key.pem
WECHAT_PAY_NOTIFY_URL=https://api.urockclimbing.cn/api/v1/payments/wechat/notify
WECHAT_REFUND_NOTIFY_URL=https://api.urockclimbing.cn/api/v1/refunds/wechat/notify
```

After the API container starts successfully with all settings present, perform these checks in order:

1. create a dedicated low-value production product;
2. enable `WECHAT_PAY_ENABLED=true` and restart only the API;
3. enable the mini-program payment entry and upload through WeChat DevTools as `Jack`;
4. complete one real payment and confirm one `PAID` order, one `SUCCEEDED` payment transaction, and exactly one issued card;
5. issue an admin refund and confirm the card moves through `REFUND_LOCKED` to `REFUNDED` after the signed notification;
6. take the low-value product off sale.

Official references:

- <https://pay.wechatpay.cn/doc/v3/merchant/4012791856>
- <https://pay.wechatpay.cn/doc/v3/merchant/4012791863>
- <https://github.com/wechatpay-apiv3/wechatpay-go>
