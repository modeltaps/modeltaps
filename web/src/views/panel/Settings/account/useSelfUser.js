import { useCallback, useEffect, useState } from 'react';

import { API } from 'utils/api';
import { showError } from 'utils/common';

// 资料 / 安全两个 section 都要读 `/api/user/self`(改名与改密码都以整份用户对象回传),
// 抽成 hook 让各 section 自持一份,互不依赖挂载顺序。

export default function useSelfUser() {
  const [inputs, setInputs] = useState({});

  const reloadUser = useCallback(async () => {
    const res = await API.get('/api/user/self');
    const { success, message, data } = res.data;
    if (success) setInputs(data);
    else showError(message);
    return success ? data : null;
  }, []);

  useEffect(() => {
    reloadUser();
  }, [reloadUser]);

  return { inputs, reloadUser };
}
