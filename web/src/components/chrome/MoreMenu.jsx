import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight, BookOpen, Ellipsis, Globe, LogOut, Monitor, Moon, Sun } from 'lucide-react';

import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import useLogin from 'hooks/useLogin';
import useTheme from 'hooks/useTheme';
import useI18n from 'hooks/useI18n';
import { setAppLanguage } from 'utils/userSetting';
import i18nList from 'i18n/i18nList';

// ==============================|| CHROME — SIDEBAR "MORE" MENU ||============================== //
// 侧栏页脚右侧的「更多」(···):文档、主题(一行分段切换)、语言、退出登录。
// 账单已是侧栏导航项,不再在这里重复。
// 照 x.ai 控制台的页脚:展开态下面板与页脚内容盒同宽、向上展开(锚点是页脚里那层无内边距的
// wrapper,所以左右各自然内缩 8px,不会溢出侧栏);折叠窄栏放不下整块面板,改为从按钮右侧弹出。
// 用户信息不在这里,身份与切换在侧栏顶部的账号行。

const THEME_OPTIONS = [
  { value: 'system', Icon: Monitor, labelKey: 'theme.auto' },
  { value: 'light', Icon: Sun, labelKey: 'theme.light' },
  { value: 'dark', Icon: Moon, labelKey: 'theme.dark' }
];

export default function MoreMenu({ anchorRef = null, collapsed = false }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { theme, setTheme } = useTheme();
  const i18n = useI18n();
  const { logout } = useLogin();

  const currentLang = i18n.language || i18nList[0].lng;
  const currentLangName = i18nList.find((item) => item.lng === currentLang)?.name || currentLang;
  const docsLink = siteInfo?.docs_link;

  // 方向键在同一层菜单内移动焦点(Esc / 外部点击由 DropdownMenu 处理)
  const onMenuKeyDown = (e) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const items = Array.from(e.currentTarget.querySelectorAll('[role="menuitem"]'));
    if (items.length === 0) return;
    const idx = items.indexOf(document.activeElement);
    const next = e.key === 'ArrowDown' ? (idx + 1) % items.length : (idx - 1 + items.length) % items.length;
    items[next].focus();
  };

  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label={t('sidebar.more', { defaultValue: 'More' })}
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <Ellipsis className="size-[1.125rem]" />
            </button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side={collapsed ? 'right' : 'top'}>{t('sidebar.more', { defaultValue: 'More' })}</TooltipContent>
      </Tooltip>
      {/* 展开态:与页脚内容盒同宽、向上展开(anchorRef 指向页脚里的无内边距 wrapper);
          折叠态:窄栏放不下,从按钮右侧弹出、底边与按钮对齐,用固定宽度。没有锚点时同样退回固定宽度 */}
      <DropdownMenuContent
        side={collapsed ? 'right' : 'top'}
        align={collapsed ? 'end' : 'start'}
        anchorRef={collapsed ? null : anchorRef}
        matchTriggerWidth={!collapsed && Boolean(anchorRef)}
        className={cn((collapsed || !anchorRef) && 'w-52')}
        onKeyDown={onMenuKeyDown}
      >
        {docsLink && (
          <DropdownMenuItem onClick={() => window.open(docsLink, '_blank', 'noopener,noreferrer')}>
            <BookOpen />
            <span className="flex-1">{t('userMenu.docs', { defaultValue: 'Docs' })}</span>
            <ArrowUpRight className="opacity-60" />
          </DropdownMenuItem>
        )}
        {docsLink && <DropdownMenuSeparator />}

        {/* 主题一行分段切换:不进二级菜单,点完菜单不关,方便连续调整 */}
        <div className="flex items-center justify-between gap-2 px-2 py-1.5 text-sm">
          <span>{t('theme.title', { defaultValue: 'Theme' })}</span>
          <div
            role="radiogroup"
            aria-label={t('theme.title', { defaultValue: 'Theme' })}
            className="flex items-center gap-0.5 rounded-md bg-muted p-0.5"
          >
            {THEME_OPTIONS.map(({ value, Icon, labelKey }) => (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={theme === value}
                aria-label={t(labelKey)}
                title={t(labelKey)}
                onClick={() => setTheme(value)}
                className={cn(
                  'flex h-6 w-7 items-center justify-center rounded-[5px] transition-colors',
                  theme === value ? 'bg-accent text-foreground' : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <Icon className="size-3.5" />
              </button>
            ))}
          </div>
        </div>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Globe />
            <span className="flex-1">{t('language.select', { defaultValue: 'Language' })}</span>
            <span className="text-xs text-muted-foreground">{currentLangName}</span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="min-w-40">
            <DropdownMenuRadioGroup value={currentLang} onValueChange={setAppLanguage}>
              {i18nList.map((item) => (
                <DropdownMenuRadioItem key={item.lng} value={item.lng}>
                  {item.name}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={logout}>
          <LogOut />
          {t('common.logout', { defaultValue: 'Log out' })}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

MoreMenu.propTypes = {
  anchorRef: PropTypes.shape({ current: PropTypes.any }),
  collapsed: PropTypes.bool
};
