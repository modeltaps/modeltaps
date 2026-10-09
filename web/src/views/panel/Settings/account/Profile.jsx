import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { UserRound } from 'lucide-react';

import { API } from 'utils/api';
import { showError, showSuccess, trims } from 'utils/common';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import useSelfUser from './useSelfUser';

// ==============================|| SETTINGS — ACCOUNT / PROFILE ||============================== //
// 头像、用户名、邮箱、用户组等只读信息 + 显示名称修改。额度三卡留在仪表盘,不在此重复。

export default function Profile() {
  const { t } = useTranslation();
  const account = useSelector((state) => state.account);
  const { inputs, reloadUser } = useSelfUser();
  const [userGroupMap, setUserGroupMap] = useState({});

  const {
    register,
    handleSubmit,
    reset,
    formState: { isDirty, isSubmitting }
  } = useForm({
    defaultValues: { display_name: '' }
  });

  useEffect(() => {
    reset({ display_name: inputs.display_name || '' });
  }, [inputs.display_name, reset]);

  useEffect(() => {
    const loadUserGroup = async () => {
      const res = await API.get('/api/user_group_map');
      if (res.data?.success) setUserGroupMap(res.data.data || {});
    };
    loadUserGroup();
  }, []);

  const onSubmit = async (values) => {
    const displayName = trims(values.display_name || '');
    try {
      const res = await API.put('/api/user/self', { ...inputs, display_name: displayName, password: '' });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('profilePage.updateSuccess'));
        reset({ display_name: displayName });
        reloadUser();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  const groupInfo = () => {
    const g = inputs.group ? userGroupMap[inputs.group] : null;
    if (!g) return <Badge variant="outline">{inputs.group || 'default'}</Badge>;
    return (
      <div className="inline-block max-w-full rounded-md border border-primary px-3 py-2 text-left">
        <p className="break-words text-sm font-semibold text-primary">{g.name}</p>
        <p className="text-xs text-muted-foreground">
          {t('profilePage.rate')}: {g.ratio} / {t('profilePage.speed')}: {g.api_rate}
        </p>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-col items-center gap-3 sm:flex-row sm:items-center sm:gap-5">
            <span className="flex size-20 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-muted">
              {account.user?.avatar_url ? (
                <img src={account.user.avatar_url} alt={inputs.username} className="size-full object-cover" />
              ) : (
                <UserRound className="h-3/4 w-3/4 text-muted-foreground" />
              )}
            </span>
            <div className="min-w-0 space-y-1 text-center sm:text-left">
              <p className="text-xl font-semibold">{inputs.username}</p>
              {inputs.email && <p className="break-words text-sm text-muted-foreground">{inputs.email}</p>}
              {inputs.phone_number && <p className="text-sm text-muted-foreground">{inputs.phone_number}</p>}
              {groupInfo()}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">{t('profilePage.personalInfo')}</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <FormField label={t('profilePage.displayName')} htmlFor="display_name">
              <Input id="display_name" placeholder={t('profilePage.inputDisplayNamePlaceholder')} {...register('display_name')} />
            </FormField>
            <div className="flex justify-end">
              <Button type="submit" disabled={!isDirty || isSubmitting}>
                {t('common.save')}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
