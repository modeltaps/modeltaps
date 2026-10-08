import OrgSettings from '../OrgSettings';

// ==============================|| SYSTEM SETTINGS — 组织策略 (root) ||============================== //
// 组织开关、每人组织上限、默认成员上限与额度转移。

export default function OrganizationTheme({ ctx }) {
  return (
    <div className="space-y-6">
      <OrgSettings ctx={ctx} />
    </div>
  );
}
