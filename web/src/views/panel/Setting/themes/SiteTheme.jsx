import { ServerAddressCard, DisplayCard, ExternalLinksCard } from '../GeneralSettings';
import { VersionNoticeCard, BrandingCard } from '../OtherSettings';
import { SmtpCard } from '../EmailSettings';

// ==============================|| SYSTEM SETTINGS — 站点 ||============================== //
// 站点地址 / 品牌 / 版本更新提示 / 显示 / 外链 / 邮件 (SMTP)。

export default function SiteTheme({ ctx }) {
  return (
    <div className="space-y-6">
      <ServerAddressCard ctx={ctx} />
      <BrandingCard ctx={ctx} />
      <VersionNoticeCard ctx={ctx} />
      <DisplayCard ctx={ctx} />
      <ExternalLinksCard ctx={ctx} />
      <SmtpCard ctx={ctx} />
    </div>
  );
}
