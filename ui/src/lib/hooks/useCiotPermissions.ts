'use client';

import {useAuth} from '@/lib/hooks/useAuth';
import {
  PERMISSION_CIOT_CREATE,
  PERMISSION_CIOT_GET,
  PERMISSION_CIOT_LIST,
  PERMISSION_CIOT_UPDATE,
} from '@/lib/constants/ciot';
import {hasPermission} from '@/lib/utils/delivery-events';

/** O que a organização selecionada pode fazer com CIOT; a api reconfere em cada rota. */
export function useCiotPermissions() {
  const {selectedOrg} = useAuth();
  const perms = selectedOrg?.permissions;
  return {
    canList: hasPermission(perms, PERMISSION_CIOT_LIST),
    canGet: hasPermission(perms, PERMISSION_CIOT_GET),
    canCreate: hasPermission(perms, PERMISSION_CIOT_CREATE),
    canUpdate: hasPermission(perms, PERMISSION_CIOT_UPDATE),
  };
}
