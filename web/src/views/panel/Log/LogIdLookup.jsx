import PropTypes from 'prop-types';
import { useState } from 'react';
import { Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { API } from 'utils/api';
import { showError } from 'utils/common';

// ==============================|| LOG — ID 查找 ||============================== //
// 粘贴日志 ID 精确定位单条并打开明细抽屉;未找到红字提示并清空输入。
// 走列表接口的 id 过滤(admin -> /api/log/;self/org -> /api/log/self/,由 orgScope 改写),
// 因此鉴权归属沿用列表口径。请求 ID(X-Modeltaps-Request-Id)不落库,故仅支持日志 ID 查找。

export default function LogIdLookup({ open, onOpenChange, effectiveAdmin, onFound, t }) {
  const [value, setValue] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async () => {
    const id = parseInt(String(value).trim(), 10);
    if (!id || loading) return;
    setLoading(true);
    try {
      const url = effectiveAdmin ? '/api/log/' : '/api/log/self/';
      const res = await API.get(url, { params: { id, page: 1, size: 1 } });
      const { success, data } = res.data;
      const item = success && data?.data?.[0];
      if (item && item.id === id) {
        onFound(item);
        onOpenChange(false);
        setValue('');
      } else {
        showError(t('logIdLookup.notFound'));
        setValue('');
      }
    } catch (error) {
      // 错误已由全局拦截器提示
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('logIdLookup.title')}</DialogTitle>
          <DialogDescription>{t('logIdLookup.description')}</DialogDescription>
        </DialogHeader>
        <DialogBody>
          <Input
            autoFocus
            type="number"
            min="1"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                submit();
              }
            }}
            placeholder={t('logIdLookup.placeholder')}
          />
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('logIdLookup.cancel')}
          </Button>
          <Button onClick={submit} disabled={loading || !String(value).trim()}>
            {loading && <Loader2 className="size-4 animate-spin" />}
            {t('logIdLookup.submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

LogIdLookup.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  effectiveAdmin: PropTypes.bool,
  onFound: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired
};
