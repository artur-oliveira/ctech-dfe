import {describe, expect, it} from 'vitest';
import {grantedMeters} from '@/lib/billing/catalog';
import {ACCOUNT_METERS} from '@/lib/constants/billing';

describe('grantedMeters', () => {
  it('has no users meter', () => {
    expect(ACCOUNT_METERS).toEqual(['companies']);
    expect(grantedMeters({nfe: 3, users: 25, companies: 10})).toEqual(['nfe', 'companies']);
  });
});
