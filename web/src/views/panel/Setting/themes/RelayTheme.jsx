import { RetryCard, ErrorHandlingCard, ModelNameMatchCard, ChannelAutoCard } from '../GeneralSettings';
import { NativeProtocolCard, PromptLimitsCard, ImageProxyCard } from '../OtherSettings';

// ==============================|| SYSTEM SETTINGS — 网关行为 ||============================== //
// 重试 / 错误处理 / 模型名匹配 / 渠道自动禁用恢复 / 原生协议 / 请求限制 / 图片代理。

export default function RelayTheme({ ctx }) {
  return (
    <div className="space-y-6">
      <RetryCard ctx={ctx} />
      <ErrorHandlingCard ctx={ctx} />
      <ModelNameMatchCard ctx={ctx} />
      <ChannelAutoCard ctx={ctx} />
      <NativeProtocolCard ctx={ctx} />
      <PromptLimitsCard ctx={ctx} />
      <ImageProxyCard ctx={ctx} />
    </div>
  );
}
