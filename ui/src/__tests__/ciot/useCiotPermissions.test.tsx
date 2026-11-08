import {renderHook} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import {useCiotPermissions} from '@/lib/hooks/useCiotPermissions';

let permissions: string[] | undefined;
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'org', permissions}})}));

const run = (p: string[] | undefined) => {
  permissions = p;
  return renderHook(() => useCiotPermissions()).result.current;
};

describe('useCiotPermissions', () => {
  it('cada ação lê a própria permissão', () => {
    expect(run(['list.ciots'])).toEqual({canList: true, canGet: false, canCreate: false, canUpdate: false});
    expect(run(['create.ciots', 'update.ciots'])).toEqual({canList: false, canGet: false, canCreate: true, canUpdate: true});
  });

  it('curinga libera tudo e a falta de permissões nega tudo', () => {
    expect(run(['*'])).toEqual({canList: true, canGet: true, canCreate: true, canUpdate: true});
    expect(run(undefined)).toEqual({canList: false, canGet: false, canCreate: false, canUpdate: false});
  });
});
