import { useMemo, useRef } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { AlertCircle, Download, Image as ImageIcon, Plus, SlidersHorizontal, TriangleAlert, X } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import ChipButton from '../components/ChipButton';
import ComposerBar from '../components/ComposerBar';
import ConsolePage from '../components/ConsolePage';
import ModelChip from '../components/ModelChip';
import NoModelsCard from '../components/NoModelsCard';
import ParamsChip from '../components/ParamsChip';
import ResultPanel from '../components/ResultPanel';
import { IMAGE_FIELDS, isChatImageModel } from '../shared/fieldSchema';
import { normalizeModel } from '../shared/modelIndex';
import ImageEmptyState from './ImageEmptyState';
import { paramsSummary } from './imageFields';
import { REFERENCE_ACCEPT } from './useImageSession';

// ==============================|| PLAYGROUND — IMAGE WORKSPACE ||============================== //
// xAI Image 页形态：结果列与贴底输入框同宽同中线，参数全部收在输入框的 chip 里（左参数、
// 右模型），页面没有右侧设置栏。还没出图时结果位置摆 2×3 模板卡，点一下填提示词与参数。
// 参考图上传只对 image-to-image 模型出现：输入框里一颗「+」图标按钮（悬停出提示），
// 选好后旁边出一颗文件名小胶囊与移除按钮。
// 对话出图模型在选择器里带「对话出图」小标签；它不吃 Images API 参数，参数 chip 不画，
// 模型附带的文字说明显示在图片上方。
// 门禁只卡发送：输入框整体置灰 + 一句说明 + 去向链接，结果与代码照常可看。

const RESULT_TABS = ['preview', 'json'];

export default function ImageWorkspace({ session, gated, gateMessage, gateActionHref, gateActionLabel, noModels }) {
  const { t } = useTranslation();
  const referenceInput = useRef(null);
  const { status, images, text = '', raw, error, fileError, reference, canUseReference, values, options, chat = false } = session;

  const running = status === 'running';
  const hasModels = options.length > 0;
  const chatBadge = t('playgroundConsole.image.chatBadge');
  const rows = useMemo(
    () => options.map((option) => ({ ...normalizeModel(option), badge: isChatImageModel(option) ? chatBadge : undefined })),
    [options, chatBadge]
  );
  const summary = paramsSummary(values);
  const hasResult = images.length > 0 || Boolean(text) || Boolean(raw);
  // 空态时模板网格与输入框一起放宽到 1000px，出图后回到 768px 的结果列宽（max-width 过渡）。
  const showTemplates = !hasResult && !running;

  const downloadAll = () => {
    images.forEach((image) => {
      const anchor = document.createElement('a');
      anchor.href = image.src;
      anchor.download = image.fileName;
      anchor.rel = 'noopener noreferrer';
      anchor.click();
    });
  };

  // 点模板：提示词进输入框，模板自带的参数补丁一并写进会话。
  const onPickExample = (item) => {
    session.setPrompt(item.prompt);
    for (const [key, value] of Object.entries(item.values || {})) session.setValue(key, value);
  };

  const composer = (
    <>
      {error && (
        <Alert variant="error" className="mb-2">
          <AlertCircle />
          <AlertDescription>
            {t(error.i18nKey)}
            {error.message ? ` — ${error.message}` : ''}
          </AlertDescription>
        </Alert>
      )}
      <ComposerBar
        value={session.prompt}
        onChange={session.setPrompt}
        placeholder={t('playgroundConsole.image.composer.placeholder')}
        running={running}
        onStop={session.abort}
        onSubmit={() => session.run()}
        disabled={gated || !session.model}
        disabledReason={
          gated || !hasModels ? (
            <span className="inline-flex items-center gap-1.5">
              <TriangleAlert className="size-3.5" />
              {gated ? gateMessage : noModels?.description}
              {(gated ? gateActionHref : noModels?.actionHref) && (
                <Link to={gated ? gateActionHref : noModels.actionHref} className="font-medium text-primary hover:underline">
                  {gated ? gateActionLabel : noModels.actionLabel}
                </Link>
              )}
            </span>
          ) : (
            fileError && <span className="text-destructive">{t(fileError)}</span>
          )
        }
        sendLabel={t('playgroundConsole.run')}
        stopLabel={t('playgroundConsole.stop')}
        leading={
          <>
            {canUseReference && (
              <>
                <TooltipProvider delayDuration={150}>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      {/* 禁用态按钮不吃指针事件，外包一层 span 让悬停提示照常出现。 */}
                      <span className="inline-flex shrink-0">
                        <ChipButton
                          caret={false}
                          className="px-2"
                          icon={<Plus className="size-4" />}
                          disabled={running}
                          aria-label={t('playgroundConsole.image.reference')}
                          onClick={() => referenceInput.current?.click()}
                        />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent className="text-xs">
                      <div>{t('playgroundConsole.image.reference')}</div>
                      <div className="text-muted-foreground">{t('playgroundConsole.image.referenceHint')}</div>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <input
                  ref={referenceInput}
                  id="playground-image-reference"
                  className="sr-only"
                  type="file"
                  tabIndex={-1}
                  aria-hidden="true"
                  accept={REFERENCE_ACCEPT}
                  disabled={running}
                  onChange={(event) => session.setReference(event.target.files?.[0] || null)}
                />
              </>
            )}
            {canUseReference && reference && (
              <span
                title={reference.name}
                className="inline-flex h-8 min-w-0 max-w-[160px] items-center gap-1.5 rounded-full border border-border px-2.5 text-xs text-foreground"
              >
                <ImageIcon className="size-3.5 shrink-0" />
                <span className="truncate">{reference.name}</span>
              </span>
            )}
            {canUseReference && reference && (
              <Button
                size="sm"
                variant="ghost"
                className="h-8 shrink-0 px-2"
                aria-label={t('playgroundConsole.image.clearReference')}
                title={t('playgroundConsole.image.clearReference')}
                onClick={() => session.setReference(null)}
              >
                <X className="size-3.5" />
              </Button>
            )}
            {!chat && (
              <ParamsChip
                schema={IMAGE_FIELDS}
                values={values}
                model={session.model}
                onChange={session.setValue}
                icon={<SlidersHorizontal className="size-3.5" />}
                summary={summary || t('playgroundConsole.image.params')}
              />
            )}
          </>
        }
        trailing={
          <ModelChip
            items={rows}
            value={session.model?.id || ''}
            disabled={!hasModels}
            label={session.model?.id || t(hasModels ? 'playgroundConsole.model.pick' : 'playgroundConsole.model.none')}
            onChange={(id) => session.setSelectedModel(id)}
          />
        }
      />
    </>
  );

  return (
    <ConsolePage
      className="min-h-0 flex-1"
      columnClassName={cn(
        'transition-[max-width] duration-300 ease-out motion-reduce:transition-none',
        showTemplates ? 'max-w-[1000px]' : 'max-w-3xl'
      )}
      composer={composer}
    >
      {showTemplates ? (
        <div className="flex flex-1 items-center justify-center py-6">
          {hasModels ? (
            <ImageEmptyState onPick={onPickExample} />
          ) : (
            <NoModelsCard
              title={noModels?.title}
              description={noModels?.description}
              actionLabel={noModels?.actionLabel}
              actionHref={noModels?.actionHref}
            />
          )}
        </div>
      ) : (
        <ResultPanel
          title={t('playgroundConsole.result.title')}
          status={status}
          statusLabel={t(`playgroundConsole.result.status.${status}`)}
          tabs={RESULT_TABS.map((id) => ({ id, label: t(`playgroundConsole.result.tabs.${id}`) }))}
          activeTab={session.resultTab}
          onTabChange={session.setResultTab}
          meta={session.meta}
          footer={
            images.length > 0 && (
              <Button size="sm" variant="outline" className="h-7" onClick={downloadAll}>
                {t('playgroundConsole.image.downloadAll')}
              </Button>
            )
          }
        >
          {session.resultTab === 'json' ? (
            <pre className="whitespace-pre-wrap break-all text-[11px] text-muted-foreground">
              {raw || t('playgroundConsole.result.empty')}
            </pre>
          ) : (
            <>
              {text && <p className="mb-2 whitespace-pre-wrap text-sm text-foreground">{text}</p>}
              <ImageGallery images={images} status={status} />
            </>
          )}
        </ResultPanel>
      )}
    </ConsolePage>
  );
}

ImageWorkspace.propTypes = {
  session: PropTypes.object.isRequired,
  gated: PropTypes.bool,
  gateMessage: PropTypes.string,
  gateActionHref: PropTypes.string,
  gateActionLabel: PropTypes.string,
  noModels: PropTypes.shape({
    title: PropTypes.string,
    description: PropTypes.string,
    actionLabel: PropTypes.string,
    actionHref: PropTypes.string
  })
};

function ImageGallery({ images, status }) {
  const { t } = useTranslation();

  if (images.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-1.5 py-10 text-xs text-muted-foreground">
        <ImageIcon className="size-5" />
        {t(
          status === 'running'
            ? 'playgroundConsole.result.waiting'
            : status === 'done'
              ? 'playgroundConsole.image.noImage'
              : 'playgroundConsole.result.empty'
        )}
      </div>
    );
  }

  return (
    <div className={images.length > 1 ? 'grid grid-cols-2 gap-2' : 'mx-auto max-w-lg'}>
      {images.map((image) => (
        <figure key={image.fileName} className="space-y-1 rounded-md border border-border p-1.5">
          <img src={image.src} alt={image.fileName} className="w-full rounded" />
          <figcaption className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
            {image.seed !== null && <span className="font-mono">seed {image.seed}</span>}
            <span className="flex-1" />
            <a
              href={image.src}
              download={image.fileName}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-primary hover:underline"
            >
              <Download className="size-3" />
              {t('playgroundConsole.image.download')}
            </a>
          </figcaption>
          {image.revisedPrompt && <p className="text-[11px] text-muted-foreground">{image.revisedPrompt}</p>}
        </figure>
      ))}
    </div>
  );
}

ImageGallery.propTypes = {
  images: PropTypes.arrayOf(
    PropTypes.shape({
      src: PropTypes.string,
      fileName: PropTypes.string,
      seed: PropTypes.any,
      revisedPrompt: PropTypes.string
    })
  ).isRequired,
  status: PropTypes.string
};
