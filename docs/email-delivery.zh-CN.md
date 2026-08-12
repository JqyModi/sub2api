# 邮箱验证码投递记录

更新时间：2026-08-12（Asia/Shanghai）

## 当前结论

验证码邮件曾被部分收件服务商放入垃圾邮件。部署 `a4f7b04` 后，用户实测邮件不再进入垃圾邮件。

目前可以确认的是：

- 发信域名的 DKIM 已配置。
- Resend 自定义 Return-Path 使用 `send.minai.eu.org`，该子域已有 SPF：`v=spf1 include:amazonses.com ~all`，并配置了 Amazon SES MX。
- `_dmarc.minai.eu.org` 已存在，当前策略为 `p=none`。
- `a4f7b04` 前，验证码邮件主要只有 HTML 内容，主题为通用的验证码标题。
- `a4f7b04` 后，邮件改为 `multipart/alternative`，同时包含纯文本和 HTML；主题中直接包含验证码；注册页面也提示用户检查垃圾邮件并标记为非垃圾邮件。

因此，当前最合理的判断是：收件服务商对新版邮件结构和内容的识别结果改善了，域名信誉积累也可能同时起了作用。由于没有保存收件箱原始邮件头和 Resend 的投递/投诉事件，不能声称某一项是唯一原因；后续若再次出现问题，应先保留完整邮件头和 Resend 事件 ID，再分析 SPF、DKIM、DMARC 对齐状态。

## 当前邮件发送约束

- 发件人：`no-reply@minai.eu.org`
- SMTP：Resend
- 验证码有效期：15 分钟
- 重发冷却：60 秒
- 单次验证码最大错误次数：5 次
- Turnstile 仍按原流程校验，没有为了提高到达率而降低注册安全限制。

## 后续维护建议

1. 不要在 `minai.eu.org` 根域新增第二条 SPF。SPF 应保持单条记录，Return-Path 子域继续由 Resend 管理。
2. 保持验证码邮件使用纯文本 + HTML 双格式，避免再次改回 HTML-only。
3. 继续使用明确的品牌发件人和验证码主题，避免频繁更换 From 地址。
4. 先保持 DMARC `p=none` 观察报告；确认所有合法发信源对齐后，再考虑逐步提高到 `quarantine`。
5. 新用户反馈收不到验证码时，优先引导检查 Spam/Junk、搜索发件人 `no-reply@minai.eu.org`，并将邮件标记为非垃圾邮件。
6. 若垃圾邮件比例再次升高，收集 Gmail/Outlook 原始邮件头、Resend 投递事件和收件时间，不要直接重复添加 DNS 记录。
