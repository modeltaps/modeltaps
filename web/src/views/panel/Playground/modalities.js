import { AudioLines, Clapperboard, Columns2, Image, MessageSquare } from 'lucide-react';

import { AudioActions, AudioCodeView, AudioProvider, AudioWorkspace } from './audio';
import { ChatActions, ChatCodeView, ChatProvider, ChatWorkspace } from './chat';
import { CompareActions, CompareCodeView, CompareProvider, CompareWorkspace } from './compare';
import { ImageActions, ImageCodeView, ImageProvider, ImageWorkspace } from './image';
import { VideoWorkspace } from './video';

// ==============================|| PLAYGROUND — MODALITY REGISTRY ||============================== //
// 分层规则(全波共享，verifier 按 import 方向检查)：
//   shared/      hook 与纯函数(useConsoleGate、ConsoleContext、后续的会话 hook / codegen /
//                fieldSchema)，禁止 import 任何 React 组件。
//   components/  只吃 props 的展示件(ConsolePage、ComposerBar、ModelPicker、CodeView、
//                FieldRenderer 等)，禁止取数、禁止读路由。
//   {chat,image,audio,compare}/  装配层，把 shared 与 components 接起来。
// 本文件是模态注册表：路由段、图标、i18n key 与各插槽(provider / workspace / settings /
// actions / codeView)。新增模态只改这里，路由页与模态栏都从这份清单生成。
// provider 是可选的装配层上下文：工作区与设置栏分处外壳两处，需要共享会话 state 的模态
// 在这里挂一个 provider，由路由页包住整个外壳。

export const DEFAULT_MODALITY = 'chat';

export const MODALITIES = [
  {
    id: 'chat',
    path: 'chat',
    icon: MessageSquare,
    labelKey: 'playgroundConsole.modality.chat',
    mainTabKey: 'playgroundConsole.tabs.chat',
    provider: ChatProvider,
    workspace: ChatWorkspace,
    // 对话页没有设置栏：参数收在输入框的 chip 里（xAI Chat 页形态）。
    // fullWidth：空态示例网格比消息列宽，列宽由工作区按「空态 / 有消息」自己收窄。
    settings: null,
    actions: ChatActions,
    codeView: ChatCodeView,
    fullWidth: true,
    comingSoon: false
  },
  {
    id: 'image',
    path: 'image',
    icon: Image,
    labelKey: 'playgroundConsole.modality.image',
    mainTabKey: 'playgroundConsole.tabs.form',
    provider: ImageProvider,
    workspace: ImageWorkspace,
    // 图像页也没有设置栏：参数收在输入框的 chip 里（xAI Image 页形态）。
    // fullWidth：同对话页，模板网格比结果列宽，列宽由工作区自己收窄。
    settings: null,
    actions: ImageActions,
    codeView: ImageCodeView,
    fullWidth: true,
    comingSoon: false
  },
  {
    id: 'audio',
    path: 'speech',
    icon: AudioLines,
    labelKey: 'playgroundConsole.modality.audio',
    mainTabKey: 'playgroundConsole.tabs.form',
    provider: AudioProvider,
    workspace: AudioWorkspace,
    // 语音页没有设置栏：参数收在编辑卡的 chip 里（xAI Voice 页形态）。
    // fullWidth：分段 tab 要与页面标题同一左边线，主区不套居中列宽，内容列由工作区自己收窄。
    settings: null,
    actions: AudioActions,
    codeView: AudioCodeView,
    fullWidth: true,
    comingSoon: false
  },
  {
    // 多列并排对比：参数收在输入框与列头的 chip 里。multiModel：各列各选模型，主区放宽到
    // 全宽，路由页也不回写 ?model=（已有的 ?model= 保留，用作第一列的默认模型）。
    id: 'compare',
    path: 'compare',
    icon: Columns2,
    labelKey: 'playgroundConsole.modality.compare',
    mainTabKey: 'playgroundConsole.tabs.chat',
    provider: CompareProvider,
    workspace: CompareWorkspace,
    settings: null,
    actions: CompareActions,
    codeView: CompareCodeView,
    multiModel: true,
    comingSoon: false
  },
  {
    // 视频还没有可运行的实现：路由可达，工作区是一张空页面(图标 + 一句话 + 接口文档入口)。
    id: 'video',
    path: 'video',
    icon: Clapperboard,
    labelKey: 'playgroundConsole.modality.video',
    mainTabKey: 'playgroundConsole.tabs.form',
    workspace: VideoWorkspace,
    settings: null,
    actions: null,
    codeView: null,
    comingSoon: true
  }
];

// 路由段 → 模态。缺省(裸 /panel/api)给默认模态；未登记的值返回 null，由路由页回落到
// 默认模态。id 与 path 不同的模态(audio ↔ speech)两种写法都认。
export function resolveModality(segment) {
  if (!segment) return MODALITIES.find((m) => m.id === DEFAULT_MODALITY);
  return MODALITIES.find((m) => m.path === segment || m.id === segment) || null;
}

export const modalityHref = (entry) => `/panel/api/${entry.path}`;

export const modalityDocsHref = (entry) => `/panel/api/docs/${entry.path}`;
