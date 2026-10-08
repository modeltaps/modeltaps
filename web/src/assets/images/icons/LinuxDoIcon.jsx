import React, { useId } from 'react';

const LinuxDoIcon = ({
  size = 20,
  color = '#ffb003',
  variant = 'default', // 'default', 'profile', 'login'
  className = '',
  style = {},
  ...props
}) => {
  // 每个实例一个稳定且唯一的 clipPath ID(useId 跨渲染不变);去掉 useId 里的冒号等符号,保证 url(#id) 可用
  const clipId = `linuxdo-clip-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;

  // 根据 variant 调整默认样式
  const getDefaultStyle = () => {
    switch (variant) {
      case 'login':
        return { marginRight: '8px', ...style };
      case 'profile':
        return { display: 'inline-block', verticalAlign: 'middle', ...style };
      default:
        return style;
    }
  };

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      style={getDefaultStyle()}
      {...props}
    >
      <clipPath id={clipId}>
        <circle cx="60" cy="60" r="47"/>
      </clipPath>
      <circle fill="#f0f0f0" cx="60" cy="60" r="50"/>
      <rect fill="#1c1c1e" clipPath={`url(#${clipId})`} x="10" y="10" width="100" height="30"/>
      <rect fill="#f0f0f0" clipPath={`url(#${clipId})`} x="10" y="40" width="100" height="40"/>
      <rect fill={color} clipPath={`url(#${clipId})`} x="10" y="80" width="100" height="30"/>
    </svg>
  );
};

export default LinuxDoIcon;
