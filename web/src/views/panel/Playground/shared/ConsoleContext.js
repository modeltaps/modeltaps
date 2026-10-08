import { createContext, useContext } from 'react';

// ==============================|| PLAYGROUND — CONSOLE CONTEXT ||============================== //
// 控制台的共享数据：目录模型、Base URL、取数状态与当前选中模型。由路由页(index.jsx)填充，
// 各模态与组件只读。本文件属 shared 层，禁止 import 任何 React 组件。

export const ConsoleContext = createContext({
  models: [],
  baseUrl: '',
  loading: false,
  error: false,
  reload: () => {},
  selectedModel: null,
  setSelectedModel: () => {}
});

export const useConsole = () => useContext(ConsoleContext);
