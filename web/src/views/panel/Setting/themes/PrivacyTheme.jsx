import { LogConsumeCard, LogIOCard, SafetyCard } from '../OtherSettings';

// ==============================|| SYSTEM SETTINGS — 日志与隐私 ||============================== //
// 消费日志 / 请求响应留存 / 内容安全。

export default function PrivacyTheme({ ctx }) {
  return (
    <div className="space-y-6">
      <LogConsumeCard ctx={ctx} />
      <LogIOCard ctx={ctx} />
      <SafetyCard ctx={ctx} />
    </div>
  );
}
