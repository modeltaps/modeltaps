import PropTypes from 'prop-types';
import { useState } from 'react';

import { cn } from '@/lib/utils';
import {
  brandIconDomainUrl,
  brandIconUploadUrl,
  brandIconUrl,
  iconCacheHost,
  resolveBrandIconKey,
  uploadIconId,
  useBrandIconManifest
} from './brandIconManifest';

// ==============================|| BRAND — ICON ||============================== //
// The single brand-mark renderer. An admin-uploaded icon (`upload:{assetId}`) renders from
// /api/brand-icon/upload/{assetId}. Otherwise it resolves a built-in icon key from the manifest
// (see resolveBrandIconKey for the order), then renders:
// - brands with a color variant as <img src="/api/brand-icon/{key}?variant=color">;
// - mono-only brands via CSS mask + currentColor so they follow the text color in dark mode;
// - the cached favicon of `domain` / the base_url host via /api/brand-icon/domain/{host};
// - `fallback` (default: a circular first-letter placeholder) when nothing resolves or loads.
// Per-brand optical `scale` from the manifest is applied as a transform on the mark only.
// Every request stays on the same origin; external icon URLs are never rendered.

function LetterPlaceholder({ fallbackText, className }) {
  const letter = (fallbackText || '').trim().charAt(0).toUpperCase() || '?';
  return (
    <span
      aria-hidden="true"
      className={cn(
        'inline-flex size-4 shrink-0 items-center justify-center rounded-full bg-muted text-[10px] font-semibold leading-none text-muted-foreground',
        className
      )}
    >
      {letter}
    </span>
  );
}

LetterPlaceholder.propTypes = {
  fallbackText: PropTypes.string,
  className: PropTypes.string
};

function Placeholder({ fallback, fallbackText, className }) {
  if (fallback !== undefined) return fallback;
  return <LetterPlaceholder fallbackText={fallbackText} className={className} />;
}

Placeholder.propTypes = {
  fallback: PropTypes.node,
  fallbackText: PropTypes.string,
  className: PropTypes.string
};

// Same-origin <img> mark that degrades to the placeholder on load error (e.g. 404).
function ImageMark({ src, dataAttrs, style, rounded, fallback, fallbackText, className }) {
  const [failed, setFailed] = useState(false);
  if (failed) return <Placeholder fallback={fallback} fallbackText={fallbackText} className={className} />;
  return (
    <img
      src={src}
      alt=""
      aria-hidden="true"
      loading="lazy"
      draggable={false}
      {...dataAttrs}
      className={cn('size-4 shrink-0 object-contain', rounded && 'rounded-sm', className)}
      style={style}
      onError={() => setFailed(true)}
    />
  );
}

ImageMark.propTypes = {
  src: PropTypes.string.isRequired,
  dataAttrs: PropTypes.object,
  style: PropTypes.object,
  rounded: PropTypes.bool,
  fallback: PropTypes.node,
  fallbackText: PropTypes.string,
  className: PropTypes.string
};

function MonoMark({ iconKey, style, className }) {
  const mask = `url("${brandIconUrl(iconKey, 'mono')}")`;
  return (
    <span
      aria-hidden="true"
      data-brand-icon={iconKey}
      className={cn('inline-block size-4 shrink-0', className)}
      style={{
        backgroundColor: 'currentColor',
        WebkitMaskImage: mask,
        maskImage: mask,
        WebkitMaskRepeat: 'no-repeat',
        maskRepeat: 'no-repeat',
        WebkitMaskPosition: 'center',
        maskPosition: 'center',
        WebkitMaskSize: 'contain',
        maskSize: 'contain',
        ...style
      }}
    />
  );
}

MonoMark.propTypes = {
  iconKey: PropTypes.string.isRequired,
  style: PropTypes.object,
  className: PropTypes.string
};

export default function BrandIcon({ icon, brandKey, ownedBy, channelType, model, baseUrl, domain, fallback, fallbackText, className }) {
  const { status, manifest } = useBrandIconManifest();
  const placeholder = { fallback, fallbackText, className };

  const uploadId = uploadIconId(icon);
  if (uploadId) {
    return (
      <ImageMark
        key={`upload:${uploadId}`}
        src={brandIconUploadUrl(uploadId)}
        dataAttrs={{ 'data-brand-upload': uploadId }}
        {...placeholder}
      />
    );
  }

  if (status === 'idle' || status === 'loading') {
    return <span aria-hidden="true" className={cn('inline-block size-4 shrink-0', className)} />;
  }

  const iconKey = resolveBrandIconKey(manifest, { icon, brandKey, ownedBy, channelType, model, baseUrl: baseUrl || domain });
  const meta = iconKey ? manifest.icons[iconKey] : null;
  if (!meta || (!meta.color && !meta.mono)) {
    const host = iconCacheHost(domain) || iconCacheHost(baseUrl);
    if (host) {
      return (
        <ImageMark
          key={`domain:${host}`}
          src={brandIconDomainUrl(host)}
          dataAttrs={{ 'data-brand-domain': host }}
          rounded
          {...placeholder}
        />
      );
    }
    return <Placeholder {...placeholder} />;
  }

  const style = meta.scale && meta.scale !== 1 ? { transform: `scale(${meta.scale})` } : undefined;
  if (meta.color) {
    return (
      <ImageMark
        key={iconKey}
        src={brandIconUrl(iconKey, 'color')}
        dataAttrs={{ 'data-brand-icon': iconKey }}
        style={style}
        {...placeholder}
      />
    );
  }
  return <MonoMark iconKey={iconKey} style={style} className={className} />;
}

BrandIcon.propTypes = {
  icon: PropTypes.string,
  brandKey: PropTypes.string,
  ownedBy: PropTypes.string,
  channelType: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  model: PropTypes.string,
  baseUrl: PropTypes.string,
  domain: PropTypes.string,
  fallback: PropTypes.node,
  fallbackText: PropTypes.string,
  className: PropTypes.string
};
