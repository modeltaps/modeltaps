import PropTypes from 'prop-types';
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, RefreshCw, RotateCcw, Upload } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Combobox } from '@/components/ui/combobox';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import BrandIcon from './BrandIcon';
import { uploadIconId, useBrandIconManifest } from './brandIconManifest';

// ==============================|| BRAND — ICON PICKER ||============================== //
// Admin control for a vendor icon value: '' (auto-resolved), `brand:{key}` (built-in icon) or
// `upload:{assetId}` (uploaded via POST /api/brand-icon/upload). The preview always renders
// through BrandIcon, so it never requests a third-party address. "Refresh" re-fetches the
// favicon of `refreshDomain` when given, otherwise runs the icon library sync now.

export const BRAND_ICON_UPLOAD_MAX_BYTES = 256 * 1024;
export const BRAND_ICON_UPLOAD_ACCEPT = 'image/svg+xml,image/png,image/jpeg,image/gif,image/webp,image/x-icon,.ico';

export function brandIconMode(value) {
  const icon = String(value ?? '').trim();
  if (!icon) return 'auto';
  if (uploadIconId(icon)) return 'upload';
  return 'builtin';
}

export async function refreshBrandIcon(domain) {
  const res = await API.post('/api/brand-icon/refresh', domain ? { domain } : {});
  return res.data;
}

export function BrandIconRefreshButton({ domain, onRefreshed, disabled }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);

  const refresh = async () => {
    setBusy(true);
    try {
      const { success, message, data } = await refreshBrandIcon(domain);
      if (!success) {
        toast.error(message);
      } else if (domain) {
        if (data?.found) toast.success(t('brandIconPicker.refreshDone', { domain: data.domain }));
        else toast.warning(t('brandIconPicker.refreshNotFound', { domain: data?.domain || domain }));
      } else {
        toast.success(t('brandIconPicker.librarySynced'));
      }
      onRefreshed?.();
    } catch {
      // API interceptor already surfaced the error.
    } finally {
      setBusy(false);
    }
  };

  return (
    <Button type="button" variant="outline" size="sm" onClick={refresh} disabled={disabled || busy}>
      {busy ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
      {t('brandIconPicker.refresh')}
    </Button>
  );
}

BrandIconRefreshButton.propTypes = {
  domain: PropTypes.string,
  onRefreshed: PropTypes.func,
  disabled: PropTypes.bool
};

export default function BrandIconPicker({ id, value, onChange, brandKey, ownedBy, domain, refreshDomain, fallbackText }) {
  const { t } = useTranslation();
  const { manifest } = useBrandIconManifest();
  const fileRef = useRef(null);
  const [uploading, setUploading] = useState(false);
  const [previewKey, setPreviewKey] = useState(0);
  const mode = brandIconMode(value);

  const options = useMemo(
    () =>
      Object.keys(manifest?.icons || {})
        .sort()
        .map((key) => ({ value: `brand:${key}`, label: key })),
    [manifest]
  );

  const upload = async (file) => {
    if (!file) return;
    if (file.size > BRAND_ICON_UPLOAD_MAX_BYTES) {
      toast.error(t('brandIconPicker.uploadTooLarge'));
      return;
    }
    const form = new FormData();
    form.append('file', file);
    setUploading(true);
    try {
      const res = await API.post('/api/brand-icon/upload', form);
      const { success, message, data } = res.data;
      if (success && data?.icon) {
        onChange(data.icon);
        toast.success(t('brandIconPicker.uploadSuccess'));
      } else {
        toast.error(message);
      }
    } catch {
      // API interceptor already surfaced the error.
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-3">
        <span className="flex size-10 shrink-0 items-center justify-center rounded-md border border-border bg-background">
          <BrandIcon
            key={previewKey}
            icon={value}
            brandKey={brandKey}
            ownedBy={ownedBy}
            domain={domain}
            fallbackText={fallbackText}
            className="size-6"
          />
        </span>
        <span className="text-sm text-muted-foreground">{t(`brandIconPicker.mode.${mode}`)}</span>
      </div>
      <Combobox
        id={id}
        value={mode === 'builtin' ? value : ''}
        onValueChange={onChange}
        options={options}
        placeholder={t('brandIconPicker.chooseBuiltin')}
        searchPlaceholder={t('brandIconPicker.searchBuiltin')}
        emptyText={t('brandIconPicker.noBuiltin')}
      />
      <div className="flex flex-wrap gap-2">
        <input
          ref={fileRef}
          type="file"
          accept={BRAND_ICON_UPLOAD_ACCEPT}
          className="hidden"
          onChange={(e) => upload(e.target.files?.[0])}
        />
        <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()} disabled={uploading}>
          {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
          {t('brandIconPicker.upload')}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => onChange('')} disabled={mode === 'auto'}>
          <RotateCcw className="size-4" />
          {t('brandIconPicker.resetAuto')}
        </Button>
        <BrandIconRefreshButton domain={refreshDomain} onRefreshed={() => setPreviewKey((k) => k + 1)} />
      </div>
    </div>
  );
}

BrandIconPicker.propTypes = {
  id: PropTypes.string,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  brandKey: PropTypes.string,
  ownedBy: PropTypes.string,
  domain: PropTypes.string,
  refreshDomain: PropTypes.string,
  fallbackText: PropTypes.string
};
